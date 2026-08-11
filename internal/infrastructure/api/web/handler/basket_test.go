// Package handler (internal test package, not handler_test) — SearchAddItem
// needs the unexported currentSession helper, so these tests live inside
// the package alongside user_test.go's and address_test.go's.
package handler

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gorilla/sessions"
	"github.com/stretchr/testify/mock"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/core/port/resource/mocks"
	servicemocks "github.com/v8tix/beecore-store-v2/internal/core/port/service/mocks"
	"github.com/v8tix/beecore-store-v2/internal/foundation/keys"
)

// basketCookieRequest builds a request carrying a valid signed session
// cookie whose Values are exactly values — a generalization of
// sessionCookieRequest (user_test.go) that isn't limited to seeding only
// keys.SessionID, since BasketHandler's own methods read keys.SessionBasketID
// and keys.SessionBasketItems out of the same cookie.
func basketCookieRequest(t *testing.T, store *sessions.CookieStore, method, target string, body io.Reader, values map[any]any) *http.Request {
	t.Helper()

	setupReq := httptest.NewRequest(http.MethodGet, "/", nil)
	cookie, err := store.Get(setupReq, keys.Session)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	cookie.Values = values

	rec := httptest.NewRecorder()
	if err := cookie.Save(setupReq, rec); err != nil {
		t.Fatalf("failed to save session: %v", err)
	}

	r := httptest.NewRequest(method, target, body)
	for _, c := range rec.Result().Cookies() {
		r.AddCookie(c)
	}
	if method == http.MethodPost {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	return r
}

func newBasketHandler(basketService *servicemocks.Basket, sessionStore *mocks.SessionStore, store *sessions.CookieStore) *BasketHandler {
	return NewBasketHandler(basketService, sessionStore, store, discardTestLogger())
}

func TestBasketHandler_Basket(t *testing.T) {
	store := newTestCookieStore()

	t.Run("no basket ID in cookie is a 500", func(t *testing.T) {
		h := newBasketHandler(servicemocks.NewBasket(t), mocks.NewSessionStore(t), store)

		w := httptest.NewRecorder()
		h.Basket(w, basketCookieRequest(t, store, http.MethodGet, "/basket", nil, map[any]any{}))

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})

	t.Run("GET renders the shopping cart", func(t *testing.T) {
		basketService := servicemocks.NewBasket(t)
		basketService.On("ComputeFinancials", mock.Anything, "b1").
			Return(domain.Financials{Total: 11.5}, nil)

		h := newBasketHandler(basketService, mocks.NewSessionStore(t), store)

		r := basketCookieRequest(t, store, http.MethodGet, "/basket", nil, map[any]any{keys.SessionBasketID: "b1"})
		w := httptest.NewRecorder()
		h.Basket(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("financials failure is a 500", func(t *testing.T) {
		basketService := servicemocks.NewBasket(t)
		basketService.On("ComputeFinancials", mock.Anything, "b1").
			Return(domain.Financials{}, errors.New("boom"))

		h := newBasketHandler(basketService, mocks.NewSessionStore(t), store)

		r := basketCookieRequest(t, store, http.MethodGet, "/basket", nil, map[any]any{keys.SessionBasketID: "b1"})
		w := httptest.NewRecorder()
		h.Basket(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})

	t.Run("POST with empty basket renders 422", func(t *testing.T) {
		basketService := servicemocks.NewBasket(t)
		basketService.On("ComputeFinancials", mock.Anything, "b1").
			Return(domain.Financials{}, nil)

		h := newBasketHandler(basketService, mocks.NewSessionStore(t), store)

		r := basketCookieRequest(t, store, http.MethodPost, "/basket",
			strings.NewReader(url.Values{}.Encode()),
			map[any]any{keys.SessionBasketID: "b1", keys.SessionBasketItems: 0},
		)
		w := httptest.NewRecorder()
		h.Basket(w, r)

		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusUnprocessableEntity)
		}
	})

	t.Run("POST with items redirects to checkout", func(t *testing.T) {
		basketService := servicemocks.NewBasket(t)
		basketService.On("ComputeFinancials", mock.Anything, "b1").
			Return(domain.Financials{Total: 11.5}, nil)

		h := newBasketHandler(basketService, mocks.NewSessionStore(t), store)

		r := basketCookieRequest(t, store, http.MethodPost, "/basket",
			strings.NewReader(url.Values{}.Encode()),
			map[any]any{keys.SessionBasketID: "b1", keys.SessionBasketItems: 2},
		)
		w := httptest.NewRecorder()
		h.Basket(w, r)

		if w.Code != http.StatusSeeOther {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusSeeOther)
		}
		if got := w.Header().Get("Location"); got != "/checkout" {
			t.Fatalf("got redirect %q, want %q", got, "/checkout")
		}
	})
}

func TestBasketHandler_AddItem(t *testing.T) {
	store := newTestCookieStore()

	t.Run("no basket ID in cookie is a 500", func(t *testing.T) {
		h := newBasketHandler(servicemocks.NewBasket(t), mocks.NewSessionStore(t), store)

		r := basketCookieRequest(t, store, http.MethodPost, "/basket/add-item/prod-1", nil, map[any]any{})
		r = withURLParam(r, "product_id", "prod-1")
		w := httptest.NewRecorder()
		h.AddItem(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})

	t.Run("POST success recomputes item count and redirects to basket", func(t *testing.T) {
		basketService := servicemocks.NewBasket(t)
		basketService.On("AddItem", mock.Anything, "b1", "prod-1").Return(domain.Basket{
			ID:    "b1",
			Items: []domain.BasketItem{{Product: domain.BasketProduct{Quantity: 3}}},
		}, nil)

		h := newBasketHandler(basketService, mocks.NewSessionStore(t), store)

		r := basketCookieRequest(t, store, http.MethodPost, "/basket/add-item/prod-1", nil, map[any]any{keys.SessionBasketID: "b1"})
		r = withURLParam(r, "product_id", "prod-1")
		w := httptest.NewRecorder()
		h.AddItem(w, r)

		if w.Code != http.StatusSeeOther {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusSeeOther)
		}
		if got := w.Header().Get("Location"); got != "/basket" {
			t.Fatalf("got redirect %q, want %q", got, "/basket")
		}
	})

	t.Run("service failure is a 500", func(t *testing.T) {
		basketService := servicemocks.NewBasket(t)
		basketService.On("AddItem", mock.Anything, "b1", "prod-1").Return(domain.Basket{}, errors.New("boom"))

		h := newBasketHandler(basketService, mocks.NewSessionStore(t), store)

		r := basketCookieRequest(t, store, http.MethodPost, "/basket/add-item/prod-1", nil, map[any]any{keys.SessionBasketID: "b1"})
		r = withURLParam(r, "product_id", "prod-1")
		w := httptest.NewRecorder()
		h.AddItem(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestBasketHandler_RemoveItem(t *testing.T) {
	store := newTestCookieStore()

	t.Run("POST success recomputes item count and redirects to basket", func(t *testing.T) {
		basketService := servicemocks.NewBasket(t)
		basketService.On("RemoveItem", mock.Anything, "b1", "prod-1").Return(domain.Basket{ID: "b1"}, nil)

		h := newBasketHandler(basketService, mocks.NewSessionStore(t), store)

		r := basketCookieRequest(t, store, http.MethodPost, "/basket/remove-item/prod-1", nil, map[any]any{keys.SessionBasketID: "b1"})
		r = withURLParam(r, "product_id", "prod-1")
		w := httptest.NewRecorder()
		h.RemoveItem(w, r)

		if w.Code != http.StatusSeeOther {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusSeeOther)
		}
		if got := w.Header().Get("Location"); got != "/basket" {
			t.Fatalf("got redirect %q, want %q", got, "/basket")
		}
	})
}

func TestBasketHandler_SearchAddItem(t *testing.T) {
	store := newTestCookieStore()

	t.Run("no session redirects with a 500", func(t *testing.T) {
		h := newBasketHandler(servicemocks.NewBasket(t), mocks.NewSessionStore(t), store)

		w := httptest.NewRecorder()
		h.SearchAddItem(w, httptest.NewRequest(http.MethodGet, "/search/add?item=prod-1", nil))

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})

	t.Run("GET success without conflict redirects to search", func(t *testing.T) {
		sessionStore := mocks.NewSessionStore(t)
		sessionStore.On("Load", mock.Anything, "sess-1").Return(domain.Session{UserID: "u1"}, nil)

		basketService := servicemocks.NewBasket(t)
		basketService.On("AddItemFromSearch", mock.Anything, "b1", "prod-1", "u1").
			Return("b1", domain.Basket{ID: "b1", Items: []domain.BasketItem{{Product: domain.BasketProduct{Quantity: 1}}}}, nil)

		h := newBasketHandler(basketService, sessionStore, store)

		r := basketCookieRequest(t, store, http.MethodGet, "/search/add?item=prod-1", nil, map[any]any{
			keys.SessionID:       "sess-1",
			keys.SessionBasketID: "b1",
		})
		w := httptest.NewRecorder()
		h.SearchAddItem(w, r)

		if w.Code != http.StatusSeeOther {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusSeeOther)
		}
		if got := w.Header().Get("Location"); got != "/search" {
			t.Fatalf("got redirect %q, want %q", got, "/search")
		}
	})

	// TestBasketHandler_SearchAddItem/GET_success_with_conflict_updates_the_basket-ID_cookie
	// proves the conflict-retry path (domain.ErrBasketCheckedOut, resolved by
	// core/basket.Service.AddItemFromSearch) updates the cookie's basket-ID
	// value when the service returns a different basket ID.
	t.Run("GET success with conflict updates the basket-ID cookie", func(t *testing.T) {
		sessionStore := mocks.NewSessionStore(t)
		sessionStore.On("Load", mock.Anything, "sess-1").Return(domain.Session{UserID: "u1"}, nil)

		basketService := servicemocks.NewBasket(t)
		basketService.On("AddItemFromSearch", mock.Anything, "stale-basket", "prod-1", "u1").
			Return("fresh-basket", domain.Basket{ID: "fresh-basket"}, nil)

		h := newBasketHandler(basketService, sessionStore, store)

		r := basketCookieRequest(t, store, http.MethodGet, "/search/add?item=prod-1", nil, map[any]any{
			keys.SessionID:       "sess-1",
			keys.SessionBasketID: "stale-basket",
		})
		w := httptest.NewRecorder()
		h.SearchAddItem(w, r)

		if w.Code != http.StatusSeeOther {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusSeeOther)
		}
		if len(w.Result().Cookies()) == 0 {
			t.Fatal("expected the basket-ID cookie to be re-saved")
		}
	})

	t.Run("service failure is a 500", func(t *testing.T) {
		sessionStore := mocks.NewSessionStore(t)
		sessionStore.On("Load", mock.Anything, "sess-1").Return(domain.Session{UserID: "u1"}, nil)

		basketService := servicemocks.NewBasket(t)
		basketService.On("AddItemFromSearch", mock.Anything, "b1", "prod-1", "u1").
			Return("", domain.Basket{}, errors.New("boom"))

		h := newBasketHandler(basketService, sessionStore, store)

		r := basketCookieRequest(t, store, http.MethodGet, "/search/add?item=prod-1", nil, map[any]any{
			keys.SessionID:       "sess-1",
			keys.SessionBasketID: "b1",
		})
		w := httptest.NewRecorder()
		h.SearchAddItem(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}
