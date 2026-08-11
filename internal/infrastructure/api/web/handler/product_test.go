package handler_test

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
	"github.com/v8tix/beecore-store-v2/internal/core/port/service/mocks"
	"github.com/v8tix/beecore-store-v2/internal/foundation/keys"
	"github.com/v8tix/beecore-store-v2/internal/infrastructure/api/web/handler"
)

func newProductHandler(productService *mocks.Product) (*handler.ProductHandler, *sessions.CookieStore) {
	store := sessions.NewCookieStore([]byte("test-secret-key-at-least-32-bytes"))
	return handler.NewProductHandler(productService, store, discardLogger()), store
}

// cookieRequestWithValues builds a request carrying a valid signed session
// cookie whose Values are exactly the given map — a generalization of
// user_test.go's sessionCookieRequest (which only ever sets keys.SessionID),
// needed here since ProductHandler reads several other cookie keys
// directly (BasketItems/UserInput/SelfPage) rather than a stored
// domain.Session.
func cookieRequestWithValues(t *testing.T, store *sessions.CookieStore, method, target string, body io.Reader, values map[any]any) *http.Request {
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

func TestProductHandler_FindProducts_GET_NoStoredState(t *testing.T) {
	h, store := newProductHandler(mocks.NewProduct(t))

	r := cookieRequestWithValues(t, store, http.MethodGet, "/search", nil, map[any]any{})
	w := httptest.NewRecorder()
	h.FindProducts(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusOK)
	}
}

func TestProductHandler_FindProducts_GET_PageParam_WithStoredSearch(t *testing.T) {
	svc := mocks.NewProduct(t)
	svc.On("Search", mock.Anything, "widget", int64(1)).
		Return(domain.ProductPage{Products: []domain.Product{{ID: "p1"}}, Total: 15}, nil)

	h, store := newProductHandler(svc)

	r := cookieRequestWithValues(t, store, http.MethodGet, "/search?page=2", nil, map[any]any{
		keys.SessionUserInput: "widget",
	})
	w := httptest.NewRecorder()
	h.FindProducts(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusOK)
	}
}

func TestProductHandler_FindProducts_GET_PageParam_NoStoredSearch(t *testing.T) {
	// Mirrors the source handler's quirk: a "page" query param with no
	// session-stored search term returns without ever writing a response
	// (httptest.ResponseRecorder defaults to 200 with an empty body in
	// that case).
	h, store := newProductHandler(mocks.NewProduct(t))

	r := cookieRequestWithValues(t, store, http.MethodGet, "/search?page=2", nil, map[any]any{})
	w := httptest.NewRecorder()
	h.FindProducts(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusOK)
	}
	if w.Body.Len() != 0 {
		t.Fatalf("expected empty body, got %q", w.Body.String())
	}
}

func TestProductHandler_FindProducts_GET_InvalidPageParam(t *testing.T) {
	h, store := newProductHandler(mocks.NewProduct(t))

	r := cookieRequestWithValues(t, store, http.MethodGet, "/search?page=notanumber", nil, map[any]any{})
	w := httptest.NewRecorder()
	h.FindProducts(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestProductHandler_FindProducts_GET_SessionStoredPage(t *testing.T) {
	svc := mocks.NewProduct(t)
	svc.On("Search", mock.Anything, "gadget", int64(2)).
		Return(domain.ProductPage{Products: []domain.Product{{ID: "p2"}}, Total: 5}, nil)

	h, store := newProductHandler(svc)

	r := cookieRequestWithValues(t, store, http.MethodGet, "/search", nil, map[any]any{
		keys.SessionSelfPage:  int64(3),
		keys.SessionUserInput: "gadget",
	})
	w := httptest.NewRecorder()
	h.FindProducts(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusOK)
	}
}

func TestProductHandler_FindProducts_POST_WithSearchTerm(t *testing.T) {
	svc := mocks.NewProduct(t)
	svc.On("Search", mock.Anything, "widget", int64(0)).
		Return(domain.ProductPage{Products: []domain.Product{{ID: "p1"}, {ID: "p2"}}, Total: 2}, nil)

	h, store := newProductHandler(svc)

	form := url.Values{"Search": {"widget"}}
	r := cookieRequestWithValues(t, store, http.MethodPost, "/search", strings.NewReader(form.Encode()), map[any]any{})
	w := httptest.NewRecorder()
	h.FindProducts(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusOK)
	}
}

func TestProductHandler_FindProducts_POST_EmptySearchTerm(t *testing.T) {
	h, store := newProductHandler(mocks.NewProduct(t))

	form := url.Values{"Search": {""}}
	r := cookieRequestWithValues(t, store, http.MethodPost, "/search", strings.NewReader(form.Encode()), map[any]any{})
	w := httptest.NewRecorder()
	h.FindProducts(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusOK)
	}
}

func TestProductHandler_FindProducts_SearchError(t *testing.T) {
	svc := mocks.NewProduct(t)
	svc.On("Search", mock.Anything, "widget", int64(0)).
		Return(domain.ProductPage{}, errors.New("boom"))

	h, store := newProductHandler(svc)

	form := url.Values{"Search": {"widget"}}
	r := cookieRequestWithValues(t, store, http.MethodPost, "/search", strings.NewReader(form.Encode()), map[any]any{})
	w := httptest.NewRecorder()
	h.FindProducts(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestProductHandler_ProductDetails_GET(t *testing.T) {
	svc := mocks.NewProduct(t)
	svc.On("FindByID", mock.Anything, "p1").Return(domain.Product{ID: "p1", Name: "Widget"}, nil)

	h, store := newProductHandler(svc)

	r := cookieRequestWithValues(t, store, http.MethodGet, "/products/p1", nil, map[any]any{})
	r = withURLParam(r, "product_id", "p1")
	w := httptest.NewRecorder()
	h.ProductDetails(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusOK)
	}
}

func TestProductHandler_ProductDetails_MissingProductID(t *testing.T) {
	h, store := newProductHandler(mocks.NewProduct(t))

	r := cookieRequestWithValues(t, store, http.MethodGet, "/products/", nil, map[any]any{})
	w := httptest.NewRecorder()
	h.ProductDetails(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestProductHandler_ProductDetails_ServiceError(t *testing.T) {
	svc := mocks.NewProduct(t)
	svc.On("FindByID", mock.Anything, "missing").Return(domain.Product{}, errors.New("boom"))

	h, store := newProductHandler(svc)

	r := cookieRequestWithValues(t, store, http.MethodGet, "/products/missing", nil, map[any]any{})
	r = withURLParam(r, "product_id", "missing")
	w := httptest.NewRecorder()
	h.ProductDetails(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusInternalServerError)
	}
}
