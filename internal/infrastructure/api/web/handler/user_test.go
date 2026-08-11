// Package handler (internal test package, not handler_test) — UpdateWizard
// needs contextSetAuthenticatedUser to seed the request context the way a
// future auth middleware eventually will, and that helper is unexported,
// so these tests live inside the package rather than alongside
// auth_test.go's external handler_test package.
package handler

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/sessions"
	"github.com/stretchr/testify/mock"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/core/port/resource/mocks"
	servicemocks "github.com/v8tix/beecore-store-v2/internal/core/port/service/mocks"
	"github.com/v8tix/beecore-store-v2/internal/foundation/keys"
)

func discardTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTestCookieStore() *sessions.CookieStore {
	return sessions.NewCookieStore([]byte("test-secret-key-at-least-32-bytes"))
}

func newUserHandler(userService *servicemocks.User, sessionStore *mocks.SessionStore, store *sessions.CookieStore) *UserHandler {
	return NewUserHandler(userService, sessionStore, store, 7*24*time.Hour, discardTestLogger())
}

// sessionCookieRequest builds a request carrying a valid signed session
// cookie whose keys.SessionID value is sessionID, as if the browser had
// already logged in — mirroring what the browser's cookie looks like
// after AuthHandler.Login runs.
func sessionCookieRequest(t *testing.T, store *sessions.CookieStore, method, target string, body io.Reader, sessionID string) *http.Request {
	t.Helper()

	setupReq := httptest.NewRequest(http.MethodGet, "/", nil)
	cookie, err := store.Get(setupReq, keys.Session)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	cookie.Values = map[any]any{keys.SessionID: sessionID}

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

func postFormRequest(t *testing.T, store *sessions.CookieStore, target string, form url.Values, sessionID string) *http.Request {
	return sessionCookieRequest(t, store, http.MethodPost, target, strings.NewReader(form.Encode()), sessionID)
}

func TestUserHandler_UpdateWizard_NoSession(t *testing.T) {
	store := newTestCookieStore()
	h := newUserHandler(servicemocks.NewUser(t), mocks.NewSessionStore(t), store)

	w := httptest.NewRecorder()
	h.UpdateWizard(w, httptest.NewRequest(http.MethodGet, "/user/update", nil))

	if w.Code != http.StatusSeeOther {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusSeeOther)
	}
	if got := w.Header().Get("Location"); got != "/login" {
		t.Fatalf("got redirect %q, want %q", got, "/login")
	}
}

func TestUserHandler_UpdateWizard_GET(t *testing.T) {
	// keys.SessionUserIN is never set anywhere in the source repo, so the
	// GET redirect branch never fires in practice — this proves the form
	// still renders even with an established session.
	t.Run("renders the form", func(t *testing.T) {
		store := newTestCookieStore()
		sessionStore := mocks.NewSessionStore(t)
		sessionStore.On("Load", mock.Anything, "sess-1").Return(domain.Session{UserID: "u1"}, nil)

		h := newUserHandler(servicemocks.NewUser(t), sessionStore, store)

		r := sessionCookieRequest(t, store, http.MethodGet, "/user/update", nil, "sess-1")
		w := httptest.NewRecorder()
		h.UpdateWizard(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusOK)
		}
	})
}

func TestUserHandler_UpdateWizard_POST(t *testing.T) {
	t.Run("validation error renders 422", func(t *testing.T) {
		store := newTestCookieStore()
		sessionStore := mocks.NewSessionStore(t)
		sessionStore.On("Load", mock.Anything, "sess-1").Return(domain.Session{UserID: "u1"}, nil)

		h := newUserHandler(servicemocks.NewUser(t), sessionStore, store)

		r := postFormRequest(t, store, "/user/update", url.Values{"IN": {""}, "PhoneNumber": {""}, "Birthday": {""}}, "sess-1")
		w := httptest.NewRecorder()
		h.UpdateWizard(w, r)

		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusUnprocessableEntity)
		}
	})

	t.Run("no authenticated user in context is a 500", func(t *testing.T) {
		store := newTestCookieStore()
		sessionStore := mocks.NewSessionStore(t)
		sessionStore.On("Load", mock.Anything, "sess-1").Return(domain.Session{UserID: "u1"}, nil)

		h := newUserHandler(servicemocks.NewUser(t), sessionStore, store)

		form := url.Values{"IN": {"0102030405"}, "PhoneNumber": {"999999999"}, "Birthday": {"1990-01-01"}}
		r := postFormRequest(t, store, "/user/update", form, "sess-1")
		w := httptest.NewRecorder()
		h.UpdateWizard(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})

	t.Run("success updates session and redirects to address wizard", func(t *testing.T) {
		store := newTestCookieStore()
		sessionStore := mocks.NewSessionStore(t)
		sessionStore.On("Load", mock.Anything, "sess-1").Return(domain.Session{UserID: "u1"}, nil)
		sessionStore.On("Save", mock.Anything, "sess-1", mock.MatchedBy(func(s domain.Session) bool {
			return s.UserID == "u1" && s.UserDNI == "0102030405"
		}), 7*24*time.Hour).Return(nil)

		userService := servicemocks.NewUser(t)
		userService.On("UpdateProfile", mock.Anything, "u1", "Jane", "Doe", "0102030405", "1990-01-01", "999999999").
			Return(nil)

		h := newUserHandler(userService, sessionStore, store)

		form := url.Values{"IN": {"0102030405"}, "PhoneNumber": {"999999999"}, "Birthday": {"1990-01-01"}}
		r := postFormRequest(t, store, "/user/update", form, "sess-1")
		r = contextSetAuthenticatedUser(r, &domain.User{ID: "u1", FirstName: "Jane", LastName: "Doe"})
		w := httptest.NewRecorder()
		h.UpdateWizard(w, r)

		if w.Code != http.StatusSeeOther {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusSeeOther)
		}
		if got := w.Header().Get("Location"); got != "/user/create/address" {
			t.Fatalf("got redirect %q, want %q", got, "/user/create/address")
		}
	})

	// TestUserHandler_UpdateWizard_POST/downstream_not_found proves the
	// source handler's 404 special-case is preserved: the session's DNI
	// is left unchanged (no Save call), but the wizard still advances to
	// the next step, matching the source's "break" (not "return") out of
	// its switch statement.
	t.Run("downstream not found still advances the wizard", func(t *testing.T) {
		store := newTestCookieStore()
		sessionStore := mocks.NewSessionStore(t)
		sessionStore.On("Load", mock.Anything, "sess-1").Return(domain.Session{UserID: "u1"}, nil)

		userService := servicemocks.NewUser(t)
		userService.On("UpdateProfile", mock.Anything, "u1", "Jane", "Doe", "0102030405", "1990-01-01", "999999999").
			Return(domain.ErrUserNotFound)

		h := newUserHandler(userService, sessionStore, store)

		form := url.Values{"IN": {"0102030405"}, "PhoneNumber": {"999999999"}, "Birthday": {"1990-01-01"}}
		r := postFormRequest(t, store, "/user/update", form, "sess-1")
		r = contextSetAuthenticatedUser(r, &domain.User{ID: "u1", FirstName: "Jane", LastName: "Doe"})
		w := httptest.NewRecorder()
		h.UpdateWizard(w, r)

		if w.Code != http.StatusSeeOther {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusSeeOther)
		}
		if got := w.Header().Get("Location"); got != "/user/create/address" {
			t.Fatalf("got redirect %q, want %q", got, "/user/create/address")
		}
		sessionStore.AssertNotCalled(t, "Save", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("generic error is a 500", func(t *testing.T) {
		store := newTestCookieStore()
		sessionStore := mocks.NewSessionStore(t)
		sessionStore.On("Load", mock.Anything, "sess-1").Return(domain.Session{UserID: "u1"}, nil)

		userService := servicemocks.NewUser(t)
		userService.On("UpdateProfile", mock.Anything, "u1", "Jane", "Doe", "0102030405", "1990-01-01", "999999999").
			Return(errors.New("boom"))

		h := newUserHandler(userService, sessionStore, store)

		form := url.Values{"IN": {"0102030405"}, "PhoneNumber": {"999999999"}, "Birthday": {"1990-01-01"}}
		r := postFormRequest(t, store, "/user/update", form, "sess-1")
		r = contextSetAuthenticatedUser(r, &domain.User{ID: "u1", FirstName: "Jane", LastName: "Doe"})
		w := httptest.NewRecorder()
		h.UpdateWizard(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}
