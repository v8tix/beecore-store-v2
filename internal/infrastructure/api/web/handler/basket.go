package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/sessions"

	httputils "github.com/v8tix/beecore-http/utils"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/core/port/resource"
	"github.com/v8tix/beecore-store-v2/internal/core/port/service"
	"github.com/v8tix/beecore-store-v2/internal/foundation/keys"
	"github.com/v8tix/beecore-store-v2/internal/foundation/web"
	"github.com/v8tix/beecore-store-v2/internal/request"
	"github.com/v8tix/beecore-store-v2/internal/response"
	"github.com/v8tix/beecore-store-v2/internal/validator"
)

// BasketHandler holds the chi handlers ported from the source repo's
// cmd/web/basket_handler.go (basket/basketAddItem/basketRemoveItem,
// reached via "/basket" and "/basket/{add,remove}-item/{product_id}")
// plus its searchAddItem handler (cmd/web/products_handler.go,
// "/search/add"). searchAddItem lives here rather than in
// ProductHandler despite its source file: its actual work —
// BasketAddItem/FindBasket/CreateBasket — is squarely a Basket-slice
// concern, and the source repo's routes.go groups its route alongside
// every other "/basket" route, not the "/search" ones (see
// ProductHandler's doc comment for the deferral this resolves).
//
// Every handler here calls port/service.Basket only; loading/saving the
// browser session cookie around that call is still handler-level
// plumbing, mirroring the source's direct session-store access, same as
// every other handler in this package.
type BasketHandler struct {
	BasketService service.Basket

	Sessions          *sessions.CookieStore
	SessionCookieName string

	// SessionStore loads the server-side domain.Session record the
	// cookie's opaque ID points at — used only by SearchAddItem, to
	// resolve the currently logged-in user's ID for its CreateBasket
	// retry path (BasketAddItem/FindBasket, this handler's other two
	// mutating methods, need only the basket ID already held in the
	// browser cookie). Same reduced (no access-token-refresh) currentSession
	// helper as UserHandler/AddressHandler; see their doc comments for why.
	SessionStore resource.SessionStore

	Logger *slog.Logger
}

func NewBasketHandler(
	basketService service.Basket,
	sessionStore resource.SessionStore,
	cookieStore *sessions.CookieStore,
	logger *slog.Logger,
) *BasketHandler {
	return &BasketHandler{
		BasketService:     basketService,
		Sessions:          cookieStore,
		SessionCookieName: keys.Session,
		SessionStore:      sessionStore,
		Logger:            logger,
	}
}

// currentSession resolves the cookie's opaque session ID and loads the
// server-side domain.Session it points at. See UserHandler.currentSession's
// doc comment for why it omits the source's access-token-refresh step.
func (h *BasketHandler) currentSession(r *http.Request) (sessionID string, sess domain.Session, ok bool) {
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

// getSessionString mirrors application.getSessionString in the source
// repo's cmd/web/helpers.go: reads key out of the browser cookie's own
// Values (not the Redis-backed domain.Session), returning ok=false if the
// key is absent, isn't a string, or is empty.
func getSessionString(session *sessions.Session, key any) (string, bool) {
	v, exists := session.Values[key]
	if !exists {
		return "", false
	}

	s, ok := v.(string)
	return s, ok && s != ""
}

// sumQuantities mirrors application.sumQuantities in the source repo's
// cmd/web/handlers.go: the total item count across every line in items,
// used to populate the browser cookie's keys.SessionBasketItems display
// value. Shared by every handler in this package that recomputes it after
// a basket mutation (this file's AddItem/RemoveItem/SearchAddItem, and
// AuthHandler.Login's basket-quantity-cookie step).
func sumQuantities(items []domain.BasketItem) int {
	var total int
	for _, item := range items {
		total += item.Product.Quantity
	}

	return total
}

// Basket mirrors the basket handler in the source repo, reached via
// GET/POST "/basket".
func (h *BasketHandler) Basket(w http.ResponseWriter, r *http.Request) {
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

	var form struct {
		FinancialItems []domain.FinancialLineItem `form:"FinancialItems"`
		IsMobile       bool                       `form:"IsMobile"`
		BasketItems    int                        `form:"BasketItems"`
		GrandTotal     float64                    `form:"GrandTotal"`
		Validator      validator.Validator        `form:"-"`
	}

	if strings.Contains(r.Header.Get("User-Agent"), "Mobile") {
		form.IsMobile = true
	}

	if items, ok := session.Values[keys.SessionBasketItems]; ok {
		if n, ok := items.(int); ok {
			form.BasketItems = n
		}
	}

	financials, err := h.BasketService.ComputeFinancials(r.Context(), basketID)
	if err != nil {
		serverError(h.Logger, w, r, err)
		return
	}
	form.FinancialItems = financials.Items
	form.GrandTotal = financials.Total

	if err := request.DecodePostForm(r, &form); err != nil {
		badRequest(h.Logger, w, r, err)
		return
	}

	switch r.Method {
	case http.MethodGet:
		data := newTemplateData(r)
		data[web.Form] = form

		if err := response.Page(w, http.StatusOK, data, "pages/shopping-cart-tmpl.html"); err != nil {
			serverError(h.Logger, w, r, err)
		}
		return

	case http.MethodPost:
		if items, ok := session.Values[keys.SessionBasketItems].(int); ok {
			if items == 0 {
				form.Validator.CheckField(false, web.BasketItems, web.MsgBasquetItemsRequired)
			}
		}

		if form.Validator.HasErrors() {
			data := newTemplateData(r)
			data[web.Form] = form

			if err := response.Page(w, http.StatusUnprocessableEntity, data, "pages/shopping-cart-tmpl.html"); err != nil {
				serverError(h.Logger, w, r, err)
			}
			return
		}

		http.Redirect(w, r, "/checkout", http.StatusSeeOther)
	}
}

// AddItem mirrors the basketAddItem handler in the source repo, reached
// via POST "/basket/add-item/{product_id}".
func (h *BasketHandler) AddItem(w http.ResponseWriter, r *http.Request) {
	productID := chi.URLParam(r, "product_id")
	if len(productID) == 0 {
		serverError(h.Logger, w, r, errors.New("product_id is required"))
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

	if r.Method == http.MethodPost {
		basket, err := h.BasketService.AddItem(r.Context(), basketID, productID)
		if err != nil {
			serverError(h.Logger, w, r, err)
			return
		}

		session.Values[keys.SessionBasketItems] = sumQuantities(basket.Items)
		if err := session.Save(r, w); err != nil {
			serverError(h.Logger, w, r, err)
			return
		}
	}

	http.Redirect(w, r, "/basket", http.StatusSeeOther)
}

// RemoveItem mirrors the basketRemoveItem handler in the source repo,
// reached via POST "/basket/remove-item/{product_id}".
func (h *BasketHandler) RemoveItem(w http.ResponseWriter, r *http.Request) {
	productID := chi.URLParam(r, "product_id")
	if len(productID) == 0 {
		serverError(h.Logger, w, r, errors.New("product_id is required"))
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

	if r.Method == http.MethodPost {
		basket, err := h.BasketService.RemoveItem(r.Context(), basketID, productID)
		if err != nil {
			serverError(h.Logger, w, r, err)
			return
		}

		session.Values[keys.SessionBasketItems] = sumQuantities(basket.Items)
		if err := session.Save(r, w); err != nil {
			serverError(h.Logger, w, r, err)
			return
		}
	}

	http.Redirect(w, r, "/basket", http.StatusSeeOther)
}

// SearchAddItem mirrors the searchAddItem handler in the source repo's
// cmd/web/products_handler.go, reached via GET "/search/add?item={product_id}"
// — see this type's doc comment for why it lives here instead of
// ProductHandler.
func (h *BasketHandler) SearchAddItem(w http.ResponseWriter, r *http.Request) {
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

	if r.Method == http.MethodGet {
		productID := httputils.GetQueryParam(r, "item")
		if len(productID) > 0 {
			if basketID, ok := getSessionString(session, keys.SessionBasketID); ok {
				newBasketID, basket, err := h.BasketService.AddItemFromSearch(r.Context(), basketID, productID, sess.UserID)
				if err != nil {
					serverError(h.Logger, w, r, err)
					return
				}

				if newBasketID != basketID {
					session.Values[keys.SessionBasketID] = newBasketID
				}
				session.Values[keys.SessionBasketItems] = sumQuantities(basket.Items)
				if err := session.Save(r, w); err != nil {
					serverError(h.Logger, w, r, err)
					return
				}
			}
		}
	}

	http.Redirect(w, r, "/search", http.StatusSeeOther)
}
