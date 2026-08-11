package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gorilla/sessions"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/core/port/resource"
	"github.com/v8tix/beecore-store-v2/internal/core/port/service"
	"github.com/v8tix/beecore-store-v2/internal/foundation/keys"
	"github.com/v8tix/beecore-store-v2/internal/foundation/web"
	"github.com/v8tix/beecore-store-v2/internal/response"
	"github.com/v8tix/beecore-store-v2/internal/validator"
)

// OrderHandler holds the chi handlers ported from the source repo's
// cmd/web/order_handler.go: confirmOrder (reached via GET
// "/orders/confirm") and cancelOrder (GET "/orders/cancel") are both
// static template renders with no repository call at all, ported
// unchanged; orders (GET "/orders", the order-history page) calls
// port/service.Order.GetByUserID only.
//
// CreateOrder/GetOrder/CancelOrder/CompleteOrder/ReadyOrder — the rest of
// orders_repository.go's method set, carried at full parity on
// resource.OrderRemote (see its doc comment) — have no handler here:
// their one live source caller (cmd/web/payphone_handlers.go's
// PayPhoneConfirm, for CreateOrder) is a Payment-slice concern, plan Task
// 17/18, not fabricated here ahead of that slice existing.
type OrderHandler struct {
	OrderService service.Order

	Sessions          *sessions.CookieStore
	SessionCookieName string

	// SessionStore loads the server-side domain.Session record the
	// cookie's opaque ID points at — used only by Orders, to resolve the
	// currently logged-in user's ID for GetByUserID. Same reduced (no
	// access-token-refresh) currentSession helper as
	// BasketHandler/UserHandler/AddressHandler; see their doc comments
	// for why.
	SessionStore resource.SessionStore

	Logger *slog.Logger
}

func NewOrderHandler(
	orderService service.Order,
	sessionStore resource.SessionStore,
	cookieStore *sessions.CookieStore,
	logger *slog.Logger,
) *OrderHandler {
	return &OrderHandler{
		OrderService:      orderService,
		Sessions:          cookieStore,
		SessionCookieName: keys.Session,
		SessionStore:      sessionStore,
		Logger:            logger,
	}
}

// currentSession resolves the cookie's opaque session ID and loads the
// server-side domain.Session it points at. See BasketHandler.currentSession's
// doc comment for why it omits the source's access-token-refresh step.
func (h *OrderHandler) currentSession(r *http.Request) (sessionID string, sess domain.Session, ok bool) {
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

// orderDisplay mirrors dto.OrderDisplay in the source repo's
// internal/business/core/dto/order_dto.go — the order-history page's
// per-row display shape: an aggregated item count (summed across
// FinancialItem.Products) and a human-formatted date, computed here at
// the handler/presentation layer rather than carried on domain.Order
// itself, same as the source's dto package kept it out of
// orders.GetOrderV1Res.
type orderDisplay struct {
	OrderID   string
	PaymentID string
	Status    string
	Amount    float64
	ItemCount int
	UpdatedAt string
}

// toOrderDisplay mirrors dto.toOrderDisplay: totalAmount comes straight
// from FinancialItem.Total (already inclusive of taxes/shipping), and
// itemCount sums each line's Quantity.
func toOrderDisplay(o domain.Order) orderDisplay {
	itemCount := 0
	for _, p := range o.FinancialItem.Products {
		itemCount += p.Quantity
	}

	return orderDisplay{
		OrderID:   o.ID,
		PaymentID: o.PaymentID,
		UpdatedAt: o.UpdatedAt.Format("02/01/2006"),
		Status:    o.Status,
		Amount:    o.FinancialItem.Total,
		ItemCount: itemCount,
	}
}

// toOrderDisplays mirrors dto.ToOrderDisplay.
func toOrderDisplays(orders []domain.Order) []orderDisplay {
	displayOrders := make([]orderDisplay, 0, len(orders))
	for _, o := range orders {
		displayOrders = append(displayOrders, toOrderDisplay(o))
	}

	return displayOrders
}

// ConfirmOrder mirrors the confirmOrder handler in the source repo,
// reached via GET "/orders/confirm" — a static template render with no
// repository call at all.
func (h *OrderHandler) ConfirmOrder(w http.ResponseWriter, r *http.Request) {
	data := newTemplateData(r)

	if err := response.Page(w, http.StatusOK, data, "pages/payment-confirmation-tmpl.html"); err != nil {
		serverError(h.Logger, w, r, err)
	}
}

// CancelOrder mirrors the cancelOrder handler in the source repo, reached
// via GET "/orders/cancel" — a static template render with no repository
// call at all.
func (h *OrderHandler) CancelOrder(w http.ResponseWriter, r *http.Request) {
	data := newTemplateData(r)

	if err := response.Page(w, http.StatusOK, data, "pages/payment-cancellation-tmpl.html"); err != nil {
		serverError(h.Logger, w, r, err)
	}
}

// Orders mirrors the orders handler in the source repo, reached via GET
// "/orders" — the logged-in user's order-history page.
func (h *OrderHandler) Orders(w http.ResponseWriter, r *http.Request) {
	_, sess, ok := h.currentSession(r)
	if !ok || sess.UserID == "" {
		serverError(h.Logger, w, r, keys.ErrSessionUserIDRequired)
		return
	}

	var form struct {
		Orders    []orderDisplay      `form:"Orders"`
		IsMobile  bool                `form:"IsMobile"`
		Validator validator.Validator `form:"-"`
	}

	if strings.Contains(r.Header.Get("User-Agent"), "Mobile") {
		form.IsMobile = true
	}

	orders, err := h.OrderService.GetByUserID(r.Context(), sess.UserID)
	if err != nil {
		// Only skip the response if the client itself is gone (checked via
		// the request context directly, not the error shape — kawa's own
		// internal per-attempt timers can also produce a "context canceled"
		// error while the client is still very much connected). Mirrors the
		// source repo's orders handler exactly.
		if r.Context().Err() != nil {
			return
		}
		serverError(h.Logger, w, r, err)
		return
	}
	form.Orders = toOrderDisplays(orders)

	data := newTemplateData(r)
	data[web.Form] = form

	if err := response.Page(w, http.StatusOK, data, "pages/orders-list-tmpl.html"); err != nil {
		serverError(h.Logger, w, r, err)
	}
}
