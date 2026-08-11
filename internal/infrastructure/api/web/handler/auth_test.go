package handler_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/sessions"
	"github.com/stretchr/testify/mock"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/core/port/service/mocks"
	"github.com/v8tix/beecore-store-v2/internal/infrastructure/api/web/handler"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newAuthHandler(authService *mocks.Auth, basketService *mocks.Basket) *handler.AuthHandler {
	store := sessions.NewCookieStore([]byte("test-secret-key-at-least-32-bytes"))
	return handler.NewAuthHandler(authService, basketService, store, discardLogger())
}

func postForm(target string, form url.Values) *http.Request {
	r := httptest.NewRequest(http.MethodPost, target, strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

// withURLParam attaches a chi URL param the way the chi router would after
// matching a "/activate/{user_id}"-style route, so handlers using
// chi.URLParam work under a direct httptest.NewRecorder call without
// standing up a full router.
func withURLParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestAuthHandler_Signup(t *testing.T) {
	t.Run("GET renders the form", func(t *testing.T) {
		h := newAuthHandler(mocks.NewAuth(t), mocks.NewBasket(t))

		w := httptest.NewRecorder()
		h.Signup(w, httptest.NewRequest(http.MethodGet, "/signup", nil))

		if w.Code != http.StatusOK {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("POST validation error renders 422", func(t *testing.T) {
		h := newAuthHandler(mocks.NewAuth(t), mocks.NewBasket(t))

		form := url.Values{"FirstName": {""}, "LastName": {""}, "Email": {""}, "Password": {""}, "ConfirmPassword": {""}}
		w := httptest.NewRecorder()
		h.Signup(w, postForm("/signup", form))

		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusUnprocessableEntity)
		}
	})

	t.Run("POST success redirects to activate", func(t *testing.T) {
		authService := mocks.NewAuth(t)
		authService.On("Signup", mock.Anything, "Jane", "Doe", "jane@b.com", "goodpassword", "goodpassword").
			Return("new-user-id", nil)
		h := newAuthHandler(authService, mocks.NewBasket(t))

		form := url.Values{
			"FirstName": {"Jane"}, "LastName": {"Doe"}, "Email": {"jane@b.com"},
			"Password": {"goodpassword"}, "ConfirmPassword": {"goodpassword"},
		}
		w := httptest.NewRecorder()
		h.Signup(w, postForm("/signup", form))

		if w.Code != http.StatusSeeOther {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusSeeOther)
		}
		if got := w.Header().Get("Location"); got != "/activate/new-user-id" {
			t.Fatalf("got redirect %q, want %q", got, "/activate/new-user-id")
		}
	})

	t.Run("POST email in use renders 422", func(t *testing.T) {
		authService := mocks.NewAuth(t)
		authService.On("Signup", mock.Anything, "Jane", "Doe", "jane@b.com", "goodpassword", "goodpassword").
			Return("", domain.ErrEmailInUse)
		h := newAuthHandler(authService, mocks.NewBasket(t))

		form := url.Values{
			"FirstName": {"Jane"}, "LastName": {"Doe"}, "Email": {"jane@b.com"},
			"Password": {"goodpassword"}, "ConfirmPassword": {"goodpassword"},
		}
		w := httptest.NewRecorder()
		h.Signup(w, postForm("/signup", form))

		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusUnprocessableEntity)
		}
	})
}

func TestAuthHandler_Login(t *testing.T) {
	t.Run("POST validation error renders 422", func(t *testing.T) {
		h := newAuthHandler(mocks.NewAuth(t), mocks.NewBasket(t))

		w := httptest.NewRecorder()
		h.Login(w, postForm("/login", url.Values{"Email": {""}, "Password": {""}}))

		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusUnprocessableEntity)
		}
	})

	t.Run("POST success sets session cookie and redirects to search", func(t *testing.T) {
		authService := mocks.NewAuth(t)
		sess := domain.Session{UserID: "u1", UserDNI: "12345", UserShippingAddress: "addr-1"}
		authService.On("Login", mock.Anything, "a@b.com", "pw").
			Return("session-id-1", sess, domain.User{ID: "u1"}, nil)

		basketService := mocks.NewBasket(t)
		basketService.On("FindByUserID", mock.Anything, "u1").Return("basket-1", nil)
		basketService.On("FindByID", mock.Anything, "basket-1").Return(domain.Basket{
			ID:    "basket-1",
			Items: []domain.BasketItem{{Product: domain.BasketProduct{Quantity: 2}}},
		}, nil)

		h := newAuthHandler(authService, basketService)

		w := httptest.NewRecorder()
		h.Login(w, postForm("/login", url.Values{"Email": {"a@b.com"}, "Password": {"pw"}}))

		if w.Code != http.StatusSeeOther {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusSeeOther)
		}
		if got := w.Header().Get("Location"); got != "/search" {
			t.Fatalf("got redirect %q, want %q", got, "/search")
		}
		if len(w.Result().Cookies()) == 0 {
			t.Fatal("expected a session cookie to be set")
		}
	})

	// TestAuthHandler_Login/POST_success_redirects_to_search_-_basket_lookup_failure_is_swallowed
	// proves the source handler's early FindBasketByUserID error-swallow
	// behavior is preserved: a failed lookup doesn't block the redirect to
	// /user/update (the DNI-missing branch here never reaches the late,
	// non-swallowed FindByID step at all).
	t.Run("POST success redirects to user update when DNI missing", func(t *testing.T) {
		authService := mocks.NewAuth(t)
		sess := domain.Session{UserID: "u1"}
		authService.On("Login", mock.Anything, "a@b.com", "pw").
			Return("session-id-1", sess, domain.User{ID: "u1"}, nil)

		basketService := mocks.NewBasket(t)
		basketService.On("FindByUserID", mock.Anything, "u1").Return("", errors.New("no basket yet"))

		h := newAuthHandler(authService, basketService)

		w := httptest.NewRecorder()
		h.Login(w, postForm("/login", url.Values{"Email": {"a@b.com"}, "Password": {"pw"}}))

		if got := w.Header().Get("Location"); got != "/user/update" {
			t.Fatalf("got redirect %q, want %q", got, "/user/update")
		}
	})

	t.Run("POST success redirects to create address when shipping address missing", func(t *testing.T) {
		authService := mocks.NewAuth(t)
		sess := domain.Session{UserID: "u1", UserDNI: "12345"}
		authService.On("Login", mock.Anything, "a@b.com", "pw").
			Return("session-id-1", sess, domain.User{ID: "u1"}, nil)

		basketService := mocks.NewBasket(t)
		basketService.On("FindByUserID", mock.Anything, "u1").Return("basket-1", nil)

		h := newAuthHandler(authService, basketService)

		w := httptest.NewRecorder()
		h.Login(w, postForm("/login", url.Values{"Email": {"a@b.com"}, "Password": {"pw"}}))

		if got := w.Header().Get("Location"); got != "/user/create/address" {
			t.Fatalf("got redirect %q, want %q", got, "/user/create/address")
		}
	})

	// TestAuthHandler_Login/POST_success_-_missing_basket_ID_at_search_redirect_is_a_500
	// proves the source handler's late basket-ID check (unlike the early
	// lookup above, not swallowed) is preserved: a fully onboarded user
	// whose basket lookup never populated the cookie hits a genuine error
	// instead of silently reaching /search with a stale item count.
	t.Run("POST success missing basket ID at search redirect is a 500", func(t *testing.T) {
		authService := mocks.NewAuth(t)
		sess := domain.Session{UserID: "u1", UserDNI: "12345", UserShippingAddress: "addr-1"}
		authService.On("Login", mock.Anything, "a@b.com", "pw").
			Return("session-id-1", sess, domain.User{ID: "u1"}, nil)

		basketService := mocks.NewBasket(t)
		basketService.On("FindByUserID", mock.Anything, "u1").Return("", errors.New("no basket yet"))

		h := newAuthHandler(authService, basketService)

		w := httptest.NewRecorder()
		h.Login(w, postForm("/login", url.Values{"Email": {"a@b.com"}, "Password": {"pw"}}))

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})

	t.Run("POST invalid credentials renders 422", func(t *testing.T) {
		authService := mocks.NewAuth(t)
		authService.On("Login", mock.Anything, "a@b.com", "wrong").
			Return("", domain.Session{}, domain.User{}, domain.ErrInvalidCredentials)
		h := newAuthHandler(authService, mocks.NewBasket(t))

		w := httptest.NewRecorder()
		h.Login(w, postForm("/login", url.Values{"Email": {"a@b.com"}, "Password": {"wrong"}}))

		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusUnprocessableEntity)
		}
	})

	t.Run("POST generic error is a 500", func(t *testing.T) {
		authService := mocks.NewAuth(t)
		authService.On("Login", mock.Anything, "a@b.com", "pw").
			Return("", domain.Session{}, domain.User{}, errors.New("boom"))
		h := newAuthHandler(authService, mocks.NewBasket(t))

		w := httptest.NewRecorder()
		h.Login(w, postForm("/login", url.Values{"Email": {"a@b.com"}, "Password": {"pw"}}))

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestAuthHandler_Activate(t *testing.T) {
	t.Run("missing user_id is a 500", func(t *testing.T) {
		h := newAuthHandler(mocks.NewAuth(t), mocks.NewBasket(t))

		w := httptest.NewRecorder()
		h.Activate(w, withURLParam(httptest.NewRequest(http.MethodGet, "/activate/", nil), "user_id", ""))

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})

	t.Run("POST success redirects", func(t *testing.T) {
		authService := mocks.NewAuth(t)
		authService.On("Activate", mock.Anything, "u1", "good-code").Return(nil)
		h := newAuthHandler(authService, mocks.NewBasket(t))

		r := withURLParam(postForm("/activate/u1", url.Values{"ActivationCode": {"good-code"}}), "user_id", "u1")
		w := httptest.NewRecorder()
		h.Activate(w, r)

		if w.Code != http.StatusSeeOther {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusSeeOther)
		}
		if got := w.Header().Get("Location"); got != "/account/activated" {
			t.Fatalf("got redirect %q, want %q", got, "/account/activated")
		}
	})

	t.Run("POST invalid token renders 422", func(t *testing.T) {
		authService := mocks.NewAuth(t)
		authService.On("Activate", mock.Anything, "u1", "bad-code").Return(domain.ErrInvalidActivationToken)
		h := newAuthHandler(authService, mocks.NewBasket(t))

		r := withURLParam(postForm("/activate/u1", url.Values{"ActivationCode": {"bad-code"}}), "user_id", "u1")
		w := httptest.NewRecorder()
		h.Activate(w, r)

		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusUnprocessableEntity)
		}
	})
}

func TestAuthHandler_ForgottenPassword(t *testing.T) {
	t.Run("POST success redirects to confirmation", func(t *testing.T) {
		authService := mocks.NewAuth(t)
		authService.On("ForgotPassword", mock.Anything, "a@b.com").Return(nil)
		h := newAuthHandler(authService, mocks.NewBasket(t))

		w := httptest.NewRecorder()
		h.ForgottenPassword(w, postForm("/password/forgot", url.Values{"Email": {"a@b.com"}}))

		if w.Code != http.StatusSeeOther {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusSeeOther)
		}
		if got := w.Header().Get("Location"); got != "/password/forgot/confirmation" {
			t.Fatalf("got redirect %q, want %q", got, "/password/forgot/confirmation")
		}
	})

	t.Run("POST unmatched email renders 422", func(t *testing.T) {
		authService := mocks.NewAuth(t)
		authService.On("ForgotPassword", mock.Anything, "missing@b.com").Return(domain.ErrUserNotFound)
		h := newAuthHandler(authService, mocks.NewBasket(t))

		w := httptest.NewRecorder()
		h.ForgottenPassword(w, postForm("/password/forgot", url.Values{"Email": {"missing@b.com"}}))

		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusUnprocessableEntity)
		}
	})
}

func TestAuthHandler_PasswordReset(t *testing.T) {
	t.Run("POST success renders confirmation page", func(t *testing.T) {
		authService := mocks.NewAuth(t)
		authService.On("ResetPassword", mock.Anything, "u1", "goodpassword", "goodpassword").Return(nil)
		h := newAuthHandler(authService, mocks.NewBasket(t))

		r := withURLParam(postForm("/password/reset/u1", url.Values{
			"Password": {"goodpassword"}, "ConfirmPassword": {"goodpassword"},
		}), "user_id", "u1")
		w := httptest.NewRecorder()
		h.PasswordReset(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("POST missing user_id is a 500", func(t *testing.T) {
		h := newAuthHandler(mocks.NewAuth(t), mocks.NewBasket(t))

		r := withURLParam(postForm("/password/reset/", url.Values{
			"Password": {"goodpassword"}, "ConfirmPassword": {"goodpassword"},
		}), "user_id", "")
		w := httptest.NewRecorder()
		h.PasswordReset(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}
