package web

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/core/port/resource/mocks"
)

func TestApp_RecoverPanic(t *testing.T) {
	t.Run("a panic is recovered as a 500 and logged", func(t *testing.T) {
		logger, buf := newCapturingLogger()
		a := &app{logger: logger}

		panicking := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("boom")
		})

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)

		a.recoverPanic(panicking).ServeHTTP(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusInternalServerError)
		}
		if !strings.Contains(buf.String(), "panic recovered") {
			t.Fatalf("got log output %q, want it to mention the recovered panic", buf.String())
		}
	})

	t.Run("no panic passes through untouched", func(t *testing.T) {
		a := &app{logger: discardLogger()}
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTeapot)
		})

		w := httptest.NewRecorder()
		a.recoverPanic(next).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

		if w.Code != http.StatusTeapot {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusTeapot)
		}
	})
}

func TestApp_SecurityHeaders(t *testing.T) {
	a := &app{}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	w := httptest.NewRecorder()
	a.securityHeaders(next).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	wantHeaders := map[string]string{
		"Referrer-Policy":        "origin-when-cross-origin",
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "deny",
		"X-XSS-Protection":       "0",
	}
	for k, want := range wantHeaders {
		if got := w.Header().Get(k); got != want {
			t.Errorf("header %s = %q, want %q", k, got, want)
		}
	}
}

func TestApp_PreventCSRF(t *testing.T) {
	a := newTestApp(t, nil, nil)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	csrfHandler := a.preventCSRF(next)

	t.Run("a safe method passes through and issues a CSRF cookie", func(t *testing.T) {
		w := httptest.NewRecorder()
		csrfHandler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

		if w.Code != http.StatusOK {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusOK)
		}

		found := false
		for _, c := range w.Result().Cookies() {
			if c.Name == "csrf_token" {
				found = true
				if !c.HttpOnly {
					t.Error("want the CSRF cookie to be HttpOnly")
				}
				if !c.Secure {
					t.Error("want the CSRF cookie to be Secure")
				}
				if c.MaxAge != int(a.sessionTTL().Seconds()) {
					t.Errorf("got MaxAge %d, want %d (session TTL)", c.MaxAge, int(a.sessionTTL().Seconds()))
				}
			}
		}
		if !found {
			t.Fatal("want a csrf_token cookie to be set")
		}
	})

	t.Run("an unsafe method without a valid token is rejected", func(t *testing.T) {
		w := httptest.NewRecorder()
		csrfHandler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/", nil))

		if w.Code != http.StatusBadRequest {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusBadRequest)
		}
	})
}

func TestApp_Authenticate(t *testing.T) {
	t.Run("no session passes through unauthenticated", func(t *testing.T) {
		a := newTestApp(t, mocks.NewAuthRemote(t), mocks.NewSessionStore(t))

		called := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			if contextGetAuthenticatedUser(r) != nil {
				t.Error("want no authenticated user in context")
			}
		})

		w := httptest.NewRecorder()
		a.authenticate(next).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

		if !called {
			t.Fatal("want the next handler to be called")
		}
	})

	t.Run("already-authenticated request skips the remote lookup", func(t *testing.T) {
		authRemote := mocks.NewAuthRemote(t) // no .On calls set — any call fails the test
		sessionStore := mocks.NewSessionStore(t)
		sess := domain.Session{UserID: "u1", UserAccessExpiry: time.Now().Add(48 * time.Hour)}
		sessionStore.On("Load", mock.Anything, "sess-1").Return(sess, nil)

		a := newTestApp(t, authRemote, sessionStore)
		called := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

		r := sessionCookieRequest(t, a.cookieStore, http.MethodGet, "/", nil, "sess-1")
		r = contextSetAuthenticatedUser(r, &domain.User{ID: "u1"})
		w := httptest.NewRecorder()
		a.authenticate(next).ServeHTTP(w, r)

		if !called {
			t.Fatal("want the next handler to be called")
		}
	})

	t.Run("valid session resolves and injects the user into context", func(t *testing.T) {
		authRemote := mocks.NewAuthRemote(t)
		sessionStore := mocks.NewSessionStore(t)
		sess := domain.Session{UserID: "u1", UserAccessExpiry: time.Now().Add(48 * time.Hour)}
		sessionStore.On("Load", mock.Anything, "sess-1").Return(sess, nil)
		authRemote.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)
		authRemote.On("FindUserByID", mock.Anything, "u1", "admin-tok").Return(domain.User{ID: "u1", Email: "a@b.com"}, nil)

		a := newTestApp(t, authRemote, sessionStore)
		var gotUser *domain.User
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotUser = contextGetAuthenticatedUser(r)
		})

		r := sessionCookieRequest(t, a.cookieStore, http.MethodGet, "/", nil, "sess-1")
		w := httptest.NewRecorder()
		a.authenticate(next).ServeHTTP(w, r)

		if gotUser == nil || gotUser.ID != "u1" {
			t.Fatalf("got %v, want user u1 injected into context", gotUser)
		}
	})

	t.Run("GetAdminToken failure is a 500", func(t *testing.T) {
		authRemote := mocks.NewAuthRemote(t)
		sessionStore := mocks.NewSessionStore(t)
		sess := domain.Session{UserID: "u1", UserAccessExpiry: time.Now().Add(48 * time.Hour)}
		sessionStore.On("Load", mock.Anything, "sess-1").Return(sess, nil)
		authRemote.On("GetAdminToken", mock.Anything).Return("", errors.New("boom"))

		a := newTestApp(t, authRemote, sessionStore)
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("next should not be called")
		})

		r := sessionCookieRequest(t, a.cookieStore, http.MethodGet, "/", nil, "sess-1")
		w := httptest.NewRecorder()
		a.authenticate(next).ServeHTTP(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})

	t.Run("FindUserByID failure is a 500", func(t *testing.T) {
		authRemote := mocks.NewAuthRemote(t)
		sessionStore := mocks.NewSessionStore(t)
		sess := domain.Session{UserID: "u1", UserAccessExpiry: time.Now().Add(48 * time.Hour)}
		sessionStore.On("Load", mock.Anything, "sess-1").Return(sess, nil)
		authRemote.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)
		authRemote.On("FindUserByID", mock.Anything, "u1", "admin-tok").Return(domain.User{}, errors.New("downstream boom"))

		a := newTestApp(t, authRemote, sessionStore)
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("next should not be called")
		})

		r := sessionCookieRequest(t, a.cookieStore, http.MethodGet, "/", nil, "sess-1")
		w := httptest.NewRecorder()
		a.authenticate(next).ServeHTTP(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})

	t.Run("a zero-value user from FindUserByID leaves the request unauthenticated", func(t *testing.T) {
		authRemote := mocks.NewAuthRemote(t)
		sessionStore := mocks.NewSessionStore(t)
		sess := domain.Session{UserID: "u1", UserAccessExpiry: time.Now().Add(48 * time.Hour)}
		sessionStore.On("Load", mock.Anything, "sess-1").Return(sess, nil)
		authRemote.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)
		authRemote.On("FindUserByID", mock.Anything, "u1", "admin-tok").Return(domain.User{}, nil)

		a := newTestApp(t, authRemote, sessionStore)
		called := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			if contextGetAuthenticatedUser(r) != nil {
				t.Error("want no authenticated user in context")
			}
		})

		r := sessionCookieRequest(t, a.cookieStore, http.MethodGet, "/", nil, "sess-1")
		w := httptest.NewRecorder()
		a.authenticate(next).ServeHTTP(w, r)

		if !called {
			t.Fatal("want the next handler to be called")
		}
	})
}

func TestApp_RequireAuthenticatedUser(t *testing.T) {
	t.Run("already-authenticated request passes through with Cache-Control", func(t *testing.T) {
		a := newTestApp(t, mocks.NewAuthRemote(t), mocks.NewSessionStore(t))
		called := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r = contextSetAuthenticatedUser(r, &domain.User{ID: "u1"})
		w := httptest.NewRecorder()
		a.requireAuthenticatedUser(next).ServeHTTP(w, r)

		if !called {
			t.Fatal("want the next handler to be called")
		}
		if got := w.Header().Get("Cache-Control"); got != "no-store" {
			t.Fatalf("got Cache-Control %q, want %q", got, "no-store")
		}
	})

	t.Run("no context user, no session redirects to login", func(t *testing.T) {
		a := newTestApp(t, mocks.NewAuthRemote(t), mocks.NewSessionStore(t))
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("next should not be called")
		})

		w := httptest.NewRecorder()
		a.requireAuthenticatedUser(next).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

		if w.Code != http.StatusSeeOther {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusSeeOther)
		}
		if got := w.Header().Get("Location"); got != "/login" {
			t.Fatalf("got redirect %q, want %q", got, "/login")
		}
	})

	t.Run("GetAdminToken failure is a 500", func(t *testing.T) {
		authRemote := mocks.NewAuthRemote(t)
		sessionStore := mocks.NewSessionStore(t)
		sess := domain.Session{UserID: "u1", UserAccessExpiry: time.Now().Add(48 * time.Hour)}
		sessionStore.On("Load", mock.Anything, "sess-1").Return(sess, nil)
		authRemote.On("GetAdminToken", mock.Anything).Return("", errors.New("boom"))

		a := newTestApp(t, authRemote, sessionStore)
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("next should not be called")
		})

		r := sessionCookieRequest(t, a.cookieStore, http.MethodGet, "/", nil, "sess-1")
		w := httptest.NewRecorder()
		a.requireAuthenticatedUser(next).ServeHTTP(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})

	t.Run("FindUserByID failure is a 500", func(t *testing.T) {
		authRemote := mocks.NewAuthRemote(t)
		sessionStore := mocks.NewSessionStore(t)
		sess := domain.Session{UserID: "u1", UserAccessExpiry: time.Now().Add(48 * time.Hour)}
		sessionStore.On("Load", mock.Anything, "sess-1").Return(sess, nil)
		authRemote.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)
		authRemote.On("FindUserByID", mock.Anything, "u1", "admin-tok").Return(domain.User{}, errors.New("downstream boom"))

		a := newTestApp(t, authRemote, sessionStore)
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("next should not be called")
		})

		r := sessionCookieRequest(t, a.cookieStore, http.MethodGet, "/", nil, "sess-1")
		w := httptest.NewRecorder()
		a.requireAuthenticatedUser(next).ServeHTTP(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})

	t.Run("valid session resolves the user and passes through with Cache-Control", func(t *testing.T) {
		authRemote := mocks.NewAuthRemote(t)
		sessionStore := mocks.NewSessionStore(t)
		sess := domain.Session{UserID: "u1", UserAccessExpiry: time.Now().Add(48 * time.Hour)}
		sessionStore.On("Load", mock.Anything, "sess-1").Return(sess, nil)
		authRemote.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)
		authRemote.On("FindUserByID", mock.Anything, "u1", "admin-tok").Return(domain.User{ID: "u1"}, nil)

		a := newTestApp(t, authRemote, sessionStore)
		var gotUser *domain.User
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotUser = contextGetAuthenticatedUser(r)
		})

		r := sessionCookieRequest(t, a.cookieStore, http.MethodGet, "/", nil, "sess-1")
		w := httptest.NewRecorder()
		a.requireAuthenticatedUser(next).ServeHTTP(w, r)

		if gotUser == nil || gotUser.ID != "u1" {
			t.Fatalf("got %v, want user u1 in context", gotUser)
		}
		if got := w.Header().Get("Cache-Control"); got != "no-store" {
			t.Fatalf("got Cache-Control %q, want %q", got, "no-store")
		}
	})

	t.Run("a zero-value user from FindUserByID redirects home", func(t *testing.T) {
		authRemote := mocks.NewAuthRemote(t)
		sessionStore := mocks.NewSessionStore(t)
		sess := domain.Session{UserID: "u1", UserAccessExpiry: time.Now().Add(48 * time.Hour)}
		sessionStore.On("Load", mock.Anything, "sess-1").Return(sess, nil)
		authRemote.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)
		authRemote.On("FindUserByID", mock.Anything, "u1", "admin-tok").Return(domain.User{}, nil)

		a := newTestApp(t, authRemote, sessionStore)
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("next should not be called")
		})

		r := sessionCookieRequest(t, a.cookieStore, http.MethodGet, "/", nil, "sess-1")
		w := httptest.NewRecorder()
		a.requireAuthenticatedUser(next).ServeHTTP(w, r)

		if w.Code != http.StatusSeeOther {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusSeeOther)
		}
		if got := w.Header().Get("Location"); got != "/" {
			t.Fatalf("got redirect %q, want %q", got, "/")
		}
	})
}

// TestApp_RequireAnonymousUser mirrors beecore-admin-v2's own middleware
// test, except the "authenticated" redirect target: this app's "/" route
// is itself guarded by requireAnonymousUser (see routes.go), so this
// middleware sends already-logged-in visitors to /search instead — see
// requireAnonymousUser's doc comment in middleware.go.
func TestApp_RequireAnonymousUser(t *testing.T) {
	t.Run("authenticated request redirects to /search", func(t *testing.T) {
		a := &app{}
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("next should not be called")
		})

		r := httptest.NewRequest(http.MethodGet, "/login", nil)
		r = contextSetAuthenticatedUser(r, &domain.User{ID: "u1"})
		w := httptest.NewRecorder()
		a.requireAnonymousUser(next).ServeHTTP(w, r)

		if w.Code != http.StatusSeeOther {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusSeeOther)
		}
		if got := w.Header().Get("Location"); got != "/search" {
			t.Fatalf("got redirect %q, want %q", got, "/search")
		}
	})

	t.Run("anonymous request passes through", func(t *testing.T) {
		a := &app{}
		called := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

		w := httptest.NewRecorder()
		a.requireAnonymousUser(next).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/login", nil))

		if !called {
			t.Fatal("want the next handler to be called")
		}
	})
}

func TestApp_Metrics(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		wantLevel string
	}{
		{"success is logged at info", http.StatusOK, "level=INFO"},
		{"client error is logged at warn", http.StatusBadRequest, "level=WARN"},
		{"server error is logged at error", http.StatusInternalServerError, "level=ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, buf := newCapturingLogger()
			a := &app{logger: logger}
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tt.status) })

			w := httptest.NewRecorder()
			a.metrics(next).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))

			if w.Code != tt.status {
				t.Fatalf("got status %d, want %d", w.Code, tt.status)
			}
			if logOutput := buf.String(); !strings.Contains(logOutput, tt.wantLevel) {
				t.Fatalf("got log output %q, want it to contain %q", logOutput, tt.wantLevel)
			}
		})
	}

	t.Run("no explicit WriteHeader call still records 200", func(t *testing.T) {
		logger, buf := newCapturingLogger()
		a := &app{logger: logger}
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// deliberately no w.WriteHeader call
		})

		w := httptest.NewRecorder()
		a.metrics(next).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))

		if !strings.Contains(buf.String(), "status=200") {
			t.Fatalf("got log output %q, want it to record status=200", buf.String())
		}
	})
}
