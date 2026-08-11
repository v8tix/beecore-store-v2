package web

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/v8tix/beecore-store-v2/internal/core/port/resource/mocks"
	"github.com/v8tix/beecore-store-v2/internal/foundation/keys"
)

// TestApp_Home differs from beecore-admin-v2's own home test: this app's
// "/" route sits behind requireAnonymousUser (see routes.go and home.go's
// doc comment), so a.home itself has no session/redirect branching left to
// exercise — a single request/response check is the whole surface.
func TestApp_Home(t *testing.T) {
	a := newTestApp(t, mocks.NewAuthRemote(t), mocks.NewSessionStore(t))

	w := httptest.NewRecorder()
	a.home(w, httptest.NewRequest(http.MethodGet, "/", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestApp_Logout(t *testing.T) {
	t.Run("an undecodable session cookie is a 500", func(t *testing.T) {
		a := newTestApp(t, mocks.NewAuthRemote(t), mocks.NewSessionStore(t))

		r := httptest.NewRequest(http.MethodGet, "/logout", nil)
		r.AddCookie(&http.Cookie{Name: keys.Session, Value: "not-a-valid-signed-cookie"})
		w := httptest.NewRecorder()
		a.logout(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})

	t.Run("no session cookie still clears the cookie and redirects", func(t *testing.T) {
		a := newTestApp(t, mocks.NewAuthRemote(t), mocks.NewSessionStore(t))

		w := httptest.NewRecorder()
		a.logout(w, httptest.NewRequest(http.MethodGet, "/logout", nil))

		if w.Code != http.StatusSeeOther {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusSeeOther)
		}
		if got := w.Header().Get("Location"); got != "/login" {
			t.Fatalf("got redirect %q, want %q", got, "/login")
		}
	})

	t.Run("a valid session deletes the server-side record and redirects", func(t *testing.T) {
		sessionStore := mocks.NewSessionStore(t)
		sessionStore.On("Delete", mock.Anything, "sess-1").Return(nil)

		a := newTestApp(t, mocks.NewAuthRemote(t), sessionStore)
		r := sessionCookieRequest(t, a.cookieStore, http.MethodGet, "/logout", nil, "sess-1")
		w := httptest.NewRecorder()
		a.logout(w, r)

		if got := w.Header().Get("Location"); got != "/login" {
			t.Fatalf("got redirect %q, want %q", got, "/login")
		}
		sessionStore.AssertCalled(t, "Delete", mock.Anything, "sess-1")
	})

	t.Run("a session delete failure is logged but logout still redirects", func(t *testing.T) {
		sessionStore := mocks.NewSessionStore(t)
		sessionStore.On("Delete", mock.Anything, "sess-1").Return(errors.New("redis down"))

		a := newTestApp(t, mocks.NewAuthRemote(t), sessionStore)
		r := sessionCookieRequest(t, a.cookieStore, http.MethodGet, "/logout", nil, "sess-1")
		w := httptest.NewRecorder()
		a.logout(w, r)

		if w.Code != http.StatusSeeOther {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusSeeOther)
		}
		if got := w.Header().Get("Location"); got != "/login" {
			t.Fatalf("got redirect %q, want %q", got, "/login")
		}
	})

	t.Run("logout clears the cookie", func(t *testing.T) {
		sessionStore := mocks.NewSessionStore(t)
		sessionStore.On("Delete", mock.Anything, "sess-1").Return(nil)

		a := newTestApp(t, mocks.NewAuthRemote(t), sessionStore)
		r := sessionCookieRequest(t, a.cookieStore, http.MethodGet, "/logout", nil, "sess-1")
		w := httptest.NewRecorder()
		a.logout(w, r)

		found := false
		for _, c := range w.Result().Cookies() {
			if c.Name == keys.Session {
				found = true
				if c.MaxAge >= 0 {
					t.Fatalf("got MaxAge %d, want a negative value to expire the cookie", c.MaxAge)
				}
			}
		}
		if !found {
			t.Fatal("want a cleared session cookie to be set on logout")
		}
	})
}
