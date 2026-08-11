// Package handler (internal test package, not handler_test) — UpdateAddress
// needs contextSetAuthenticatedUser and contextGetAuthenticatedUser, and
// CreateAddressWizard/NewAddress/UpdateAddress all need the unexported
// currentSession helper, so these tests live inside the package alongside
// user_test.go's.
package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/mock"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/core/port/resource/mocks"
	servicemocks "github.com/v8tix/beecore-store-v2/internal/core/port/service/mocks"
)

// withURLParam attaches a chi URL param the way the chi router would after
// matching an "/addresses/{address_id}"-style route, so handlers using
// chi.URLParam work under a direct httptest.NewRecorder call without
// standing up a full router. Duplicated from auth_test.go's identically
// named helper: that one lives in the external handler_test package,
// this file lives in the internal handler package (needs
// contextSetAuthenticatedUser), so the two can't share it directly.
func withURLParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestAddressHandler_NewAddress(t *testing.T) {
	store := newTestCookieStore()

	t.Run("no session is a 500", func(t *testing.T) {
		h := NewAddressHandler(servicemocks.NewAddress(t), mocks.NewSessionStore(t), store, 7*24*time.Hour, discardTestLogger())

		w := httptest.NewRecorder()
		h.NewAddress(w, httptest.NewRequest(http.MethodGet, "/addresses/new", nil))

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})

	t.Run("GET renders the form", func(t *testing.T) {
		sessionStore := mocks.NewSessionStore(t)
		sessionStore.On("Load", mock.Anything, "sess-1").Return(domain.Session{UserID: "u1"}, nil)

		h := NewAddressHandler(servicemocks.NewAddress(t), sessionStore, store, 7*24*time.Hour, discardTestLogger())

		r := sessionCookieRequest(t, store, http.MethodGet, "/addresses/new", nil, "sess-1")
		w := httptest.NewRecorder()
		h.NewAddress(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("POST validation error renders 422", func(t *testing.T) {
		sessionStore := mocks.NewSessionStore(t)
		sessionStore.On("Load", mock.Anything, "sess-1").Return(domain.Session{UserID: "u1"}, nil)

		h := NewAddressHandler(servicemocks.NewAddress(t), sessionStore, store, 7*24*time.Hour, discardTestLogger())

		r := postFormRequest(t, store, "/addresses/new", url.Values{}, "sess-1")
		w := httptest.NewRecorder()
		h.NewAddress(w, r)

		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusUnprocessableEntity)
		}
	})

	validForm := url.Values{
		"Country": {"EC"}, "State": {"Pichincha"}, "City": {"Quito"}, "ZIP": {"170101"},
		"MainAddress": {"Main St"}, "SecondaryAddress": {"2nd St"}, "Numeration": {"123"}, "Phone": {"999999999"},
	}

	t.Run("POST success redirects to checkout", func(t *testing.T) {
		sessionStore := mocks.NewSessionStore(t)
		sessionStore.On("Load", mock.Anything, "sess-1").Return(domain.Session{UserID: "u1"}, nil)

		addressService := servicemocks.NewAddress(t)
		addressService.On("Insert", mock.Anything, "u1", domain.ShippingAddress, "EC", "Quito", "Pichincha", "170101", "Main St", "2nd St", "123", "999999999").
			Return("new-addr", nil)

		h := NewAddressHandler(addressService, sessionStore, store, 7*24*time.Hour, discardTestLogger())

		r := postFormRequest(t, store, "/addresses/new", validForm, "sess-1")
		w := httptest.NewRecorder()
		h.NewAddress(w, r)

		if w.Code != http.StatusSeeOther {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusSeeOther)
		}
		if got := w.Header().Get("Location"); got != "/checkout" {
			t.Fatalf("got redirect %q, want %q", got, "/checkout")
		}
	})

	t.Run("POST max addresses reached flags the field", func(t *testing.T) {
		sessionStore := mocks.NewSessionStore(t)
		sessionStore.On("Load", mock.Anything, "sess-1").Return(domain.Session{UserID: "u1"}, nil)

		addressService := servicemocks.NewAddress(t)
		addressService.On("Insert", mock.Anything, "u1", domain.ShippingAddress, "EC", "Quito", "Pichincha", "170101", "Main St", "2nd St", "123", "999999999").
			Return("", domain.ErrMaxAddressesReached)

		h := NewAddressHandler(addressService, sessionStore, store, 7*24*time.Hour, discardTestLogger())

		r := postFormRequest(t, store, "/addresses/new", validForm, "sess-1")
		w := httptest.NewRecorder()
		h.NewAddress(w, r)

		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusUnprocessableEntity)
		}
	})

	t.Run("POST bad address type is a 400", func(t *testing.T) {
		sessionStore := mocks.NewSessionStore(t)
		sessionStore.On("Load", mock.Anything, "sess-1").Return(domain.Session{UserID: "u1"}, nil)

		addressService := servicemocks.NewAddress(t)
		addressService.On("Insert", mock.Anything, "u1", domain.ShippingAddress, "EC", "Quito", "Pichincha", "170101", "Main St", "2nd St", "123", "999999999").
			Return("", domain.ErrBadAddressType)

		h := NewAddressHandler(addressService, sessionStore, store, 7*24*time.Hour, discardTestLogger())

		r := postFormRequest(t, store, "/addresses/new", validForm, "sess-1")
		w := httptest.NewRecorder()
		h.NewAddress(w, r)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusBadRequest)
		}
	})
}

func TestAddressHandler_UpdateAddress(t *testing.T) {
	store := newTestCookieStore()

	t.Run("missing address_id is a 500", func(t *testing.T) {
		h := NewAddressHandler(servicemocks.NewAddress(t), mocks.NewSessionStore(t), store, 7*24*time.Hour, discardTestLogger())

		r := withURLParam(httptest.NewRequest(http.MethodGet, "/addresses/", nil), "address_id", "")
		w := httptest.NewRecorder()
		h.UpdateAddress(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})

	t.Run("no authenticated user in context is a 500", func(t *testing.T) {
		sessionStore := mocks.NewSessionStore(t)
		sessionStore.On("Load", mock.Anything, "sess-1").Return(domain.Session{UserID: "u1"}, nil)

		h := NewAddressHandler(servicemocks.NewAddress(t), sessionStore, store, 7*24*time.Hour, discardTestLogger())

		r := sessionCookieRequest(t, store, http.MethodGet, "/addresses/addr-1", nil, "sess-1")
		r = withURLParam(r, "address_id", "addr-1")
		w := httptest.NewRecorder()
		h.UpdateAddress(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})

	t.Run("GET pre-populates the form from the matching address", func(t *testing.T) {
		sessionStore := mocks.NewSessionStore(t)
		sessionStore.On("Load", mock.Anything, "sess-1").Return(domain.Session{UserID: "u1"}, nil)

		addressService := servicemocks.NewAddress(t)
		addressService.On("FindByUserID", mock.Anything, "u1").Return([]domain.Address{
			{ID: "addr-1", Country: "EC", Phone: "+593999999999"},
			{ID: "addr-2", Country: "US"},
		}, nil)

		h := NewAddressHandler(addressService, sessionStore, store, 7*24*time.Hour, discardTestLogger())

		r := sessionCookieRequest(t, store, http.MethodGet, "/addresses/addr-1", nil, "sess-1")
		r = withURLParam(r, "address_id", "addr-1")
		r = contextSetAuthenticatedUser(r, &domain.User{ID: "u1"})
		w := httptest.NewRecorder()
		h.UpdateAddress(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("POST success redirects to checkout", func(t *testing.T) {
		sessionStore := mocks.NewSessionStore(t)
		sessionStore.On("Load", mock.Anything, "sess-1").Return(domain.Session{UserID: "u1"}, nil)

		addressService := servicemocks.NewAddress(t)
		addressService.On("FindByUserID", mock.Anything, "u1").Return([]domain.Address{{ID: "addr-1"}}, nil)
		addressService.On("Update", mock.Anything, "addr-1", "u1", domain.ShippingAddress, "EC", "Quito", "Pichincha", "170101", "Main St", "2nd St", "123", "999999999").
			Return(nil)

		h := NewAddressHandler(addressService, sessionStore, store, 7*24*time.Hour, discardTestLogger())

		form := url.Values{
			"Country": {"EC"}, "State": {"Pichincha"}, "City": {"Quito"}, "ZIP": {"170101"},
			"MainAddress": {"Main St"}, "SecondaryAddress": {"2nd St"}, "Numeration": {"123"}, "Phone": {"999999999"},
		}
		r := postFormRequest(t, store, "/addresses/addr-1", form, "sess-1")
		r = withURLParam(r, "address_id", "addr-1")
		r = contextSetAuthenticatedUser(r, &domain.User{ID: "u1"})
		w := httptest.NewRecorder()
		h.UpdateAddress(w, r)

		if w.Code != http.StatusSeeOther {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusSeeOther)
		}
		if got := w.Header().Get("Location"); got != "/checkout" {
			t.Fatalf("got redirect %q, want %q", got, "/checkout")
		}
	})
}

func TestAddressHandler_CreateAddressWizard(t *testing.T) {
	store := newTestCookieStore()

	t.Run("no session redirects to login", func(t *testing.T) {
		h := NewAddressHandler(servicemocks.NewAddress(t), mocks.NewSessionStore(t), store, 7*24*time.Hour, discardTestLogger())

		w := httptest.NewRecorder()
		h.CreateAddressWizard(w, httptest.NewRequest(http.MethodGet, "/user/create/address", nil))

		if got := w.Header().Get("Location"); got != "/login" {
			t.Fatalf("got redirect %q, want %q", got, "/login")
		}
	})

	t.Run("GET redirects to user update when DNI missing", func(t *testing.T) {
		sessionStore := mocks.NewSessionStore(t)
		sessionStore.On("Load", mock.Anything, "sess-1").Return(domain.Session{UserID: "u1"}, nil)

		h := NewAddressHandler(servicemocks.NewAddress(t), sessionStore, store, 7*24*time.Hour, discardTestLogger())

		r := sessionCookieRequest(t, store, http.MethodGet, "/user/create/address", nil, "sess-1")
		w := httptest.NewRecorder()
		h.CreateAddressWizard(w, r)

		if got := w.Header().Get("Location"); got != "/user/update" {
			t.Fatalf("got redirect %q, want %q", got, "/user/update")
		}
	})

	t.Run("GET redirects to search when shipping address already set", func(t *testing.T) {
		sessionStore := mocks.NewSessionStore(t)
		sessionStore.On("Load", mock.Anything, "sess-1").Return(domain.Session{UserID: "u1", UserDNI: "12345", UserShippingAddress: "addr-1"}, nil)

		h := NewAddressHandler(servicemocks.NewAddress(t), sessionStore, store, 7*24*time.Hour, discardTestLogger())

		r := sessionCookieRequest(t, store, http.MethodGet, "/user/create/address", nil, "sess-1")
		w := httptest.NewRecorder()
		h.CreateAddressWizard(w, r)

		if got := w.Header().Get("Location"); got != "/search" {
			t.Fatalf("got redirect %q, want %q", got, "/search")
		}
	})

	t.Run("GET renders the form", func(t *testing.T) {
		sessionStore := mocks.NewSessionStore(t)
		sessionStore.On("Load", mock.Anything, "sess-1").Return(domain.Session{UserID: "u1", UserDNI: "12345"}, nil)

		h := NewAddressHandler(servicemocks.NewAddress(t), sessionStore, store, 7*24*time.Hour, discardTestLogger())

		r := sessionCookieRequest(t, store, http.MethodGet, "/user/create/address", nil, "sess-1")
		w := httptest.NewRecorder()
		h.CreateAddressWizard(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("POST success saves session and redirects to done", func(t *testing.T) {
		sessionStore := mocks.NewSessionStore(t)
		sessionStore.On("Load", mock.Anything, "sess-1").Return(domain.Session{UserID: "u1", UserDNI: "12345"}, nil)
		sessionStore.On("Save", mock.Anything, "sess-1", mock.MatchedBy(func(s domain.Session) bool {
			return s.UserShippingAddress == "new-addr"
		}), 7*24*time.Hour).Return(nil)

		addressService := servicemocks.NewAddress(t)
		addressService.On("Insert", mock.Anything, "u1", domain.ShippingAddress, "EC", "Quito", "Pichincha", "170101", "Main St", "2nd St", "123", "999999999").
			Return("new-addr", nil)

		h := NewAddressHandler(addressService, sessionStore, store, 7*24*time.Hour, discardTestLogger())

		form := url.Values{
			"Country": {"EC"}, "State": {"Pichincha"}, "City": {"Quito"}, "ZIP": {"170101"},
			"MainAddress": {"Main St"}, "SecondaryAddress": {"2nd St"}, "Numeration": {"123"}, "Phone": {"999999999"},
		}
		r := postFormRequest(t, store, "/user/create/address", form, "sess-1")
		w := httptest.NewRecorder()
		h.CreateAddressWizard(w, r)

		if w.Code != http.StatusSeeOther {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusSeeOther)
		}
		if got := w.Header().Get("Location"); got != "/user/update/done" {
			t.Fatalf("got redirect %q, want %q", got, "/user/update/done")
		}
	})

	t.Run("POST generic error is a 500", func(t *testing.T) {
		sessionStore := mocks.NewSessionStore(t)
		sessionStore.On("Load", mock.Anything, "sess-1").Return(domain.Session{UserID: "u1", UserDNI: "12345"}, nil)

		addressService := servicemocks.NewAddress(t)
		addressService.On("Insert", mock.Anything, "u1", domain.ShippingAddress, "EC", "Quito", "Pichincha", "170101", "Main St", "2nd St", "123", "999999999").
			Return("", errors.New("boom"))

		h := NewAddressHandler(addressService, sessionStore, store, 7*24*time.Hour, discardTestLogger())

		form := url.Values{
			"Country": {"EC"}, "State": {"Pichincha"}, "City": {"Quito"}, "ZIP": {"170101"},
			"MainAddress": {"Main St"}, "SecondaryAddress": {"2nd St"}, "Numeration": {"123"}, "Phone": {"999999999"},
		}
		r := postFormRequest(t, store, "/user/create/address", form, "sess-1")
		w := httptest.NewRecorder()
		h.CreateAddressWizard(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestAddressHandler_DoneWizard(t *testing.T) {
	h := NewAddressHandler(servicemocks.NewAddress(t), mocks.NewSessionStore(t), newTestCookieStore(), 7*24*time.Hour, discardTestLogger())

	w := httptest.NewRecorder()
	h.DoneWizard(w, httptest.NewRequest(http.MethodGet, "/user/update/done", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusOK)
	}
}
