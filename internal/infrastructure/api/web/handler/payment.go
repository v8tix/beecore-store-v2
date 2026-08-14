package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gorilla/sessions"

	httputils "github.com/v8tix/beecore-http/utils"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/core/port/resource"
	"github.com/v8tix/beecore-store-v2/internal/core/port/service"
	"github.com/v8tix/beecore-store-v2/internal/foundation/keys"
	"github.com/v8tix/beecore-store-v2/internal/foundation/web"
	"github.com/v8tix/beecore-store-v2/internal/request"
	"github.com/v8tix/beecore-store-v2/internal/response"
	"github.com/v8tix/beecore-store-v2/internal/token"
	"github.com/v8tix/beecore-store-v2/internal/validator"
)

// PaymentHandler holds the chi handlers ported from the PayPhone sections
// of cmd/web/payphone_handlers.go in the source repo — PayPhoneCreatePayment
// (ProcessPayment here), PayPhoneConfirm (ConfirmPayment), PayPhoneCancel
// (CancelPayment) — plus checkoutPayPhone (cmd/web/payphone_handlers.go,
// reached via GET/POST "/checkout" — checkout_handler.go's thin
// PaymentStrategy dispatch is dropped along with PayPal, see spec
// assumption #5) as Checkout. PayPal is deprecated and not ported; this is
// the only payment handler in this repo, and every method below calls
// port/service.Payment only (plus, for ProcessPayment/Checkout,
// port/service.Basket, and for Checkout, port/service.Address — mirroring
// AuthHandler's/UserHandler's existing precedent for composing a second
// service).
//
// One deliberate scope reduction remains vs. the source repo: no
// payment-record persistence against the downstream beecore-payments
// service (source's CreatePayment/FindPayment/UpdatePayment in
// cmd/web/payphone_handlers.go's PayPhoneCreatePayment/PayPhoneConfirm/
// PayPhoneCancel) — that's the app's own audit/idempotency record-keeping
// for a payment attempt, entirely separate from the PayPhone gateway call
// itself (resource.PayPhoneRemote, fully ported) and from the order/basket
// orchestration below. Needs its own port/resource.PaymentRecordRemote +
// adapter + service wiring; tracked here rather than silently dropped.
//
// ConfirmPayment's per-store order creation + basket checkout + fresh
// basket (source: PayPhoneConfirm's CreateOrder-per-store + CheckoutBasket
// + CreateBasket sequence) IS wired — core/payment.Service depends on
// resource.OrderRemote/resource.BasketRemote directly for it (the same
// "services depend on resources, not other services" rule core/auth.Login
// already applies to resource.AddressRemote), not through
// service.Order/service.Basket.
//
// The PayPhone gateway integration itself — ProcessPayment/ConfirmPayment/
// CancelPayment against pay.payphonetodoesposible.com, and Checkout's
// GeneratePayPhoneConfig-based widget config — is fully wired and preserves
// source behavior.
type PaymentHandler struct {
	PaymentService service.Payment

	// BasketService resolves ProcessPayment's and Checkout's financial
	// breakdown (ComputeFinancials) — the source repo's
	// PayPhoneCreatePayment/checkoutPayPhone handlers do the same via
	// app.repo.ComputeFinancials before building their canonical payment
	// request. Not needed by ConfirmPayment/CancelPayment, which never
	// touch a basket.
	BasketService service.Basket

	// AddressService resolves Checkout's shipping-address list (source's
	// checkoutPayPhone handler's FindAddressesByUserID call). Not needed by
	// any other method here.
	AddressService service.Address

	// SessionStore loads the server-side domain.Session record the
	// cookie's opaque ID points at — used only by Checkout, to resolve the
	// currently logged-in user's ID/access-token/DNI. Same reduced (no
	// access-token-refresh) currentSession helper as
	// BasketHandler/OrderHandler/UserHandler/AddressHandler; see their doc
	// comments for why.
	SessionStore resource.SessionStore

	// Currency and TaxesPercentage mirror the two
	// app.Cfg.BusinessParameters.Country values the source repo's
	// checkoutPayPhone/PayPhoneCreatePayment handlers read directly —
	// resolved once at composition-root time (plan Task 19) and injected
	// here, the same pattern UserHandler/AddressHandler already use for
	// SessionTTL, since core stays free of any *config.Cfg import.
	Currency        string
	TaxesPercentage int

	Sessions          *sessions.CookieStore
	SessionCookieName string
	Logger            *slog.Logger
}

func NewPaymentHandler(
	paymentService service.Payment,
	basketService service.Basket,
	addressService service.Address,
	sessionStore resource.SessionStore,
	currency string,
	taxesPercentage int,
	cookieStore *sessions.CookieStore,
	logger *slog.Logger,
) *PaymentHandler {
	return &PaymentHandler{
		PaymentService:    paymentService,
		BasketService:     basketService,
		AddressService:    addressService,
		SessionStore:      sessionStore,
		Currency:          currency,
		TaxesPercentage:   taxesPercentage,
		Sessions:          cookieStore,
		SessionCookieName: keys.Session,
		Logger:            logger,
	}
}

// currentSession resolves the cookie's opaque session ID and loads the
// server-side domain.Session it points at. See BasketHandler.currentSession's
// doc comment for why it omits the source's access-token-refresh step.
func (h *PaymentHandler) currentSession(r *http.Request) (sessionID string, sess domain.Session, ok bool) {
	cookie, err := h.Sessions.Get(r, h.SessionCookieName)
	if err != nil {
		return "", domain.Session{}, false
	}

	v, exists := cookie.Values[keys.SessionID]
	if !exists {
		return "", domain.Session{}, false
	}

	sessionID, ok = v.(string)
	if !ok || sessionID == "" {
		return "", domain.Session{}, false
	}

	sess, err = h.SessionStore.Load(r.Context(), sessionID)
	if err != nil {
		return sessionID, domain.Session{}, false
	}

	return sessionID, sess, true
}

// processPaymentRequest is ProcessPayment's AJAX request body — sent by
// assets/static/js/payphone/checkout-payphone.js's createPaymentRecord once
// the shopper picks a shipping address. Total is accepted but unused here;
// the server always recomputes the authoritative total via BasketService.
type processPaymentRequest struct {
	ShippingAddressID string  `json:"shipping_address_id"`
	Total             float64 `json:"total"`
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
// page's AJAX caller. Also stashes the request's shipping address ID in
// the session (see processPaymentRequest's doc comment) — ConfirmPayment
// needs it later to build orders and check the basket out.
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

	var body processPaymentRequest
	if err := request.DecodeJSON(w, r, &body); err != nil {
		badRequest(h.Logger, w, r, err)
		return
	}
	if body.ShippingAddressID == "" {
		badRequest(h.Logger, w, r, keys.ErrMissingShippingAddress)
		return
	}

	// Stashed here (rather than resolved from the checkout form later) so
	// ConfirmPayment — reached via PayPhone's own redirect back to this
	// app, not a normal form submission — can still build one Order per
	// store without the shopper's original request. Mirrors the source
	// repo's PayPhoneCreatePayment handler's identical
	// cookie.Values[keys.SessionShippingAddressID] assignment.
	session.Values[keys.SessionShippingAddressID] = body.ShippingAddressID
	if err := session.Save(r, w); err != nil {
		serverError(h.Logger, w, r, err)
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

// ConfirmPayment mirrors the PayPhoneConfirm handler in the source repo.
// Reads the "id"/"clientTransactionId" query parameters PayPhone's own
// redirect back to this route carries, resolves userID/basketID/
// paymentID/shippingAddressID from the session, and calls
// PaymentService.ConfirmPayment — which, only on an approved payment,
// creates the resulting orders, checks the basket out, and opens a fresh
// one (see port/service.Payment.ConfirmPayment's doc comment). A
// non-approved confirmation (domain.PaymentConfirmation.ErrorMessage set)
// is treated as a failure, same as the source's ErrPaymentNotApproved
// handling — newBasketID is empty in that case, so the session's basket
// state is deliberately left untouched, leaving the basket available to
// retry.
func (h *PaymentHandler) ConfirmPayment(w http.ResponseWriter, r *http.Request) {
	payphoneID := httputils.GetQueryParam(r, "id")
	clientTransactionID := httputils.GetQueryParam(r, keys.PayPhoneClientTransactionIDParam)

	_, sess, ok := h.currentSession(r)
	if !ok || sess.UserID == "" {
		serverError(h.Logger, w, r, keys.ErrSessionUserIDRequired)
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

	shippingAddressID, ok := getSessionString(session, keys.SessionShippingAddressID)
	if !ok {
		serverError(h.Logger, w, r, keys.ErrMissingShippingAddress)
		return
	}

	// Mirrors the source handler's own fallback: the payment ID set by
	// ProcessPayment should already be in session by the time PayPhone
	// redirects back here, but fall back to the client transaction ID
	// (the value PayPhone itself echoes back) if it's somehow missing
	// rather than failing outright.
	paymentID, ok := getSessionString(session, keys.SessionPaymentID)
	if !ok {
		paymentID = clientTransactionID
	}

	confirmation, newBasketID, err := h.PaymentService.ConfirmPayment(
		r.Context(), payphoneID, clientTransactionID, sess.UserID, basketID, paymentID, shippingAddressID,
	)
	if err != nil {
		serverError(h.Logger, w, r, err)
		return
	}

	if confirmation.ErrorMessage != "" {
		serverError(h.Logger, w, r, errors.New(confirmation.ErrorMessage))
		return
	}

	// Mirrors the source's cookie.Values[keys.SessionPayPhoneConfirmation]
	// assignment: only the transaction ID is kept in the browser cookie
	// (full confirmation details would exceed its size limits), matching
	// the source's own comment on that assignment.
	session.Values[keys.SessionPayPhoneConfirmation] = confirmation.TransactionID
	// The basket ConfirmPayment just checked out is done — point the
	// session at the fresh one it opened so the shopper's cart is empty
	// on their next visit instead of showing the order they just paid
	// for. Losing this step is exactly what "the basket never
	// reinitiates" looks like.
	session.Values[keys.SessionBasketID] = newBasketID
	session.Values[keys.SessionBasketItems] = 0
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

// checkoutForm is Checkout's form, ported field-for-field from the
// anonymous struct in the source repo's checkoutPayPhone handler —
// Addresses holds []domain.Address instead of []users.Address,
// FinancialItems holds []domain.FinancialLineItem instead of
// []common.FinancialItemV1.
type checkoutForm struct {
	FinancialItems    []domain.FinancialLineItem `form:"FinancialItems"`
	Addresses         []domain.Address           `form:"Addresses"`
	BasketItems       int                        `form:"BasketItems"`
	BasketSubtotal    float64                    `form:"BasketSubtotal"`
	TaxesPercentage   int                        `form:"TaxesPercentage"`
	Taxes             float64                    `form:"Taxes"`
	Shipping          float64                    `form:"Shipping"`
	Total             float64                    `form:"Total"`
	IsMobile          bool                       `form:"IsMobile"`
	ShippingAddressID string                     `form:"ShippingAddress"`
	PayPhoneConfig    map[string]any             `form:"-"`
	Validator         validator.Validator        `form:"-"`
}

// Checkout mirrors the checkoutPayPhone handler in the source repo's
// cmd/web/payphone_handlers.go, reached via GET/POST "/checkout" (the
// source's checkout_handler.go's PaymentStrategy dispatch has nothing left
// to dispatch on now that PayPal is gone — see this type's doc comment).
//
// GET renders the checkout page with the basket's financial breakdown, the
// user's saved addresses, and — once there's at least one product to pay
// for — a freshly minted client transaction ID (persisted into the session
// cookie, replacing any leftover confirmation/payment/order state from a
// prior attempt) and the PayPhone JS-widget config built from it. POST
// validates the shopper picked a non-empty basket and a shipping address;
// on success it stores the client transaction ID as this attempt's payment
// ID and redirects back to "/checkout" — mirrors the source handler's own
// comment that, for PayPhone's inline-widget integration, this POST branch
// isn't actually hit during normal use (the JS widget drives
// ProcessPayment/ConfirmPayment directly).
func (h *PaymentHandler) Checkout(w http.ResponseWriter, r *http.Request) {
	var form checkoutForm

	if strings.Contains(r.Header.Get("User-Agent"), "Mobile") {
		form.IsMobile = true
	}

	_, sess, ok := h.currentSession(r)
	if !ok || sess.UserAccessToken == "" {
		serverError(h.Logger, w, r, keys.ErrUserAccessTokenRequired)
		return
	}
	if sess.UserID == "" {
		serverError(h.Logger, w, r, keys.ErrSessionUserIDRequired)
		return
	}

	cookie, err := h.Sessions.Get(r, h.SessionCookieName)
	if err != nil {
		serverError(h.Logger, w, r, err)
		return
	}

	basketID, ok := getSessionString(cookie, keys.SessionBasketID)
	if !ok {
		serverError(h.Logger, w, r, keys.ErrSessionBasketIDRequired)
		return
	}

	form.TaxesPercentage = h.TaxesPercentage

	addresses, err := h.AddressService.FindByUserID(r.Context(), sess.UserID)
	if err != nil {
		serverError(h.Logger, w, r, err)
		return
	}
	form.Addresses = addresses

	financials, err := h.BasketService.ComputeFinancials(r.Context(), basketID)
	if err != nil {
		serverError(h.Logger, w, r, err)
		return
	}
	form.FinancialItems = financials.Items
	form.BasketSubtotal = financials.Subtotal
	form.Shipping = financials.TotalShippingAmount
	form.Taxes = financials.TotalTaxesAmount
	form.Total = financials.Total

	if items, ok := cookie.Values[keys.SessionBasketItems].(int); ok {
		form.BasketItems = items
	}

	paymentReq := domain.PaymentRequest{
		UserID:     sess.UserID,
		Currency:   h.Currency,
		Financials: financials,
	}

	switch r.Method {
	case http.MethodGet:
		if len(financials.Items) > 0 {
			clientTransactionID, err := token.New()
			if err != nil {
				serverError(h.Logger, w, r, err)
				return
			}

			// Clean up leftover state from a prior checkout attempt before
			// storing this one's — mirrors the source handler exactly.
			delete(cookie.Values, keys.SessionPayPhoneConfirmation)
			delete(cookie.Values, keys.SessionPaymentID)
			delete(cookie.Values, keys.SessionOrderID)
			cookie.Values[keys.SessionClientTransactionID] = clientTransactionID

			payphoneConfig, err := h.PaymentService.GeneratePayPhoneConfigWithTransactionID(r.Context(), paymentReq, clientTransactionID)
			if err != nil {
				serverError(h.Logger, w, r, err)
				return
			}
			form.PayPhoneConfig = payphoneConfig

			if err := cookie.Save(r, w); err != nil {
				serverError(h.Logger, w, r, err)
				return
			}
		}

		data := newTemplateData(r)
		data[web.Form] = form

		if err := response.Page(w, http.StatusOK, data, "pages/checkout-payphone-integrated-tmpl.html"); err != nil {
			serverError(h.Logger, w, r, err)
		}
		return

	case http.MethodPost:
		if err := request.DecodePostForm(r, &form); err != nil {
			badRequest(h.Logger, w, r, err)
			return
		}

		if items, ok := cookie.Values[keys.SessionBasketItems].(int); ok {
			if items == 0 {
				form.Validator.CheckField(false, web.BasketItems, web.MsgBasquetItemsRequired)
			}
		}
		form.Validator.CheckField(len(form.ShippingAddressID) != 0, web.ShippingAddress, web.MsgShippingAddressRequired)

		clientTransactionID, ok := getSessionString(cookie, keys.SessionClientTransactionID)
		if !ok {
			serverError(h.Logger, w, r, keys.ErrClientTransactionIDRequired)
			return
		}
		paymentReq.ClientTransactionID = clientTransactionID

		if form.Validator.HasErrors() {
			payphoneConfig, err := h.PaymentService.GeneratePayPhoneConfig(r.Context(), paymentReq)
			if err != nil {
				serverError(h.Logger, w, r, err)
				return
			}
			form.PayPhoneConfig = payphoneConfig

			data := newTemplateData(r)
			data[web.Form] = form

			if err := response.Page(w, http.StatusUnprocessableEntity, data, "pages/checkout-payphone-integrated-tmpl.html"); err != nil {
				serverError(h.Logger, w, r, err)
			}
			return
		}

		// NOTE: for PayPhone's inline-widget integration, payment records
		// are created via AJAX (ProcessPayment) once a shipping address is
		// picked — this POST handler isn't hit in normal use; see this
		// method's doc comment.
		delete(cookie.Values, keys.SessionPayPhoneConfirmation)
		delete(cookie.Values, keys.SessionOrderID)
		cookie.Values[keys.SessionPaymentID] = clientTransactionID

		if err := cookie.Save(r, w); err != nil {
			serverError(h.Logger, w, r, err)
			return
		}

		http.Redirect(w, r, "/checkout", http.StatusSeeOther)
	}
}
