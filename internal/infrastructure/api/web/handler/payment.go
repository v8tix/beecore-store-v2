package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gorilla/sessions"

	httputils "github.com/v8tix/beecore-http/utils"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/core/port/service"
	"github.com/v8tix/beecore-store-v2/internal/foundation/keys"
	"github.com/v8tix/beecore-store-v2/internal/response"
	"github.com/v8tix/beecore-store-v2/internal/token"
)

// PaymentHandler holds the chi handlers ported from the PayPhone sections
// of cmd/web/payphone_handlers.go in the source repo — PayPhoneCreatePayment
// (ProcessPayment here), PayPhoneConfirm (ConfirmPayment) and PayPhoneCancel
// (CancelPayment). PayPal is deprecated and not ported (spec assumption
// #5); this is the only payment handler in this repo, and every method
// below calls port/service.Payment only (plus, for ProcessPayment,
// port/service.Basket — see its doc comment — mirroring AuthHandler's/
// UserHandler's existing precedent for composing a second service).
//
// Reduced scope vs. the source repo, matching core/payment.Service's own
// reduced scope (see resource.PayPhoneRemote's doc comment):
//   - No PayPhone JS-widget config generation (source's checkoutPayPhone
//     GET / GeneratePayPhoneConfig) — needs FindUserByID, not available on
//     any port yet (deferred to the Task 19 composition-root middleware).
//   - No payment-record persistence against the downstream beecore-payments
//     service (source's CreatePayment/FindPayment/UpdatePayment) — no port
//     for it exists; the spec's resource list and Success Criteria name
//     only auth/user/store/address/product/basket/order/payphone.
//   - No per-store order creation / basket checkout orchestration (source's
//     PayPhoneConfirm handler's CreateOrder-per-store + CheckoutBasket +
//     CreateBasket sequence) — out of scope for this task pair; a future
//     caller can compose service.Payment with service.Order/service.Basket
//     the same way this handler already composes service.Basket for
//     ProcessPayment.
type PaymentHandler struct {
	PaymentService service.Payment

	// BasketService resolves ProcessPayment's financial breakdown
	// (ComputeFinancials) — the source repo's PayPhoneCreatePayment
	// handler does the same via app.repo.ComputeFinancials before
	// building its canonical payment request. Not needed by
	// ConfirmPayment/CancelPayment, which never touch a basket.
	BasketService service.Basket

	Sessions          *sessions.CookieStore
	SessionCookieName string
	Logger            *slog.Logger
}

func NewPaymentHandler(
	paymentService service.Payment,
	basketService service.Basket,
	cookieStore *sessions.CookieStore,
	logger *slog.Logger,
) *PaymentHandler {
	return &PaymentHandler{
		PaymentService:    paymentService,
		BasketService:     basketService,
		Sessions:          cookieStore,
		SessionCookieName: keys.Session,
		Logger:            logger,
	}
}

// ProcessPayment mirrors the PayPhoneCreatePayment handler in the source
// repo, reduced to its port/service.Payment call (see this type's doc
// comment for what's out of scope): reads the session's basket ID,
// computes its financial breakdown via BasketService, mints a fresh
// order/client-transaction identity pair (source: uuid.New() for the
// order ID and BaseRepositoryImpl.GenerateTransactionID for the client
// transaction ID — neither ported, see domain.PaymentRequest's doc
// comment for why the two are kept distinct; internal/token.New() is this
// repo's existing crypto-random ID generator, used the same way session
// IDs are), and calls PaymentService.ProcessPayment. Responds with
// domain.PaymentResult (confirm/cancel URLs) as JSON, for the checkout
// page's AJAX caller.
func (h *PaymentHandler) ProcessPayment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	session, err := h.Sessions.Get(r, h.SessionCookieName)
	if err != nil {
		serverError(h.Logger, w, r, err)
		return
	}

	basketID, ok := getSessionString(session, keys.SessionBasketID)
	if !ok {
		serverError(h.Logger, w, r, keys.ErrSessionBasketIDRequired)
		return
	}

	financials, err := h.BasketService.ComputeFinancials(r.Context(), basketID)
	if err != nil {
		serverError(h.Logger, w, r, err)
		return
	}

	orderID, err := token.New()
	if err != nil {
		serverError(h.Logger, w, r, err)
		return
	}

	clientTransactionID, err := token.New()
	if err != nil {
		serverError(h.Logger, w, r, err)
		return
	}

	result, err := h.PaymentService.ProcessPayment(r.Context(), domain.PaymentRequest{
		OrderID:             orderID,
		ClientTransactionID: clientTransactionID,
		Financials:          financials,
	})
	if err != nil {
		serverError(h.Logger, w, r, err)
		return
	}

	// Persisted so ConfirmPayment/CancelPayment's callers (PayPhone's own
	// redirect back to this app) can be correlated with this attempt, same
	// as the source repo's cookie.Values[keys.SessionClientTransactionID]/
	// SessionPaymentID assignments in checkoutPayPhone/PayPhoneCreatePayment.
	session.Values[keys.SessionClientTransactionID] = clientTransactionID
	session.Values[keys.SessionPaymentID] = result.PaymentID
	if err := session.Save(r, w); err != nil {
		serverError(h.Logger, w, r, err)
		return
	}

	if err := response.JSON(w, http.StatusOK, result); err != nil {
		serverError(h.Logger, w, r, err)
	}
}

// ConfirmPayment mirrors the PayPhoneConfirm handler in the source repo,
// reduced to its port/service.Payment call (see this type's doc comment
// for what's out of scope). Reads the "id"/"clientTransactionId" query
// parameters PayPhone's own redirect back to this route carries, and
// calls PaymentService.ConfirmPayment. A non-approved confirmation
// (domain.PaymentConfirmation.ErrorMessage set) is treated as a failure,
// same as the source's ErrPaymentNotApproved handling.
func (h *PaymentHandler) ConfirmPayment(w http.ResponseWriter, r *http.Request) {
	payphoneID := httputils.GetQueryParam(r, "id")
	clientTransactionID := httputils.GetQueryParam(r, keys.PayPhoneClientTransactionIDParam)

	confirmation, err := h.PaymentService.ConfirmPayment(r.Context(), payphoneID, clientTransactionID)
	if err != nil {
		serverError(h.Logger, w, r, err)
		return
	}

	if confirmation.ErrorMessage != "" {
		serverError(h.Logger, w, r, errors.New(confirmation.ErrorMessage))
		return
	}

	session, err := h.Sessions.Get(r, h.SessionCookieName)
	if err != nil {
		serverError(h.Logger, w, r, err)
		return
	}

	// Mirrors the source's cookie.Values[keys.SessionPayPhoneConfirmation]
	// assignment: only the transaction ID is kept in the browser cookie
	// (full confirmation details would exceed its size limits), matching
	// the source's own comment on that assignment.
	session.Values[keys.SessionPayPhoneConfirmation] = confirmation.TransactionID
	if err := session.Save(r, w); err != nil {
		serverError(h.Logger, w, r, err)
		return
	}

	http.Redirect(w, r, "/orders/confirm", http.StatusSeeOther)
}

// CancelPayment mirrors the PayPhoneCancel handler in the source repo,
// reduced to its port/service.Payment call (see this type's doc comment
// for what's out of scope). Reads the same "id"/"clientTransactionId"
// query parameters ConfirmPayment does and calls
// PaymentService.CancelPayment — a deliberate no-op today, see
// resource.PayPhoneRemote's doc comment.
func (h *PaymentHandler) CancelPayment(w http.ResponseWriter, r *http.Request) {
	payphoneID := httputils.GetQueryParam(r, "id")
	clientTransactionID := httputils.GetQueryParam(r, keys.PayPhoneClientTransactionIDParam)

	if err := h.PaymentService.CancelPayment(r.Context(), payphoneID, clientTransactionID); err != nil {
		serverError(h.Logger, w, r, err)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
