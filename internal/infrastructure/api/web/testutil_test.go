package web

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/sessions"

	"github.com/v8tix/beecore-eda/config"

	"github.com/v8tix/beecore-store-v2/internal/core/port/resource"
	"github.com/v8tix/beecore-store-v2/internal/foundation/keys"
)

// discardLogger is a *slog.Logger that throws every record away — for
// tests that exercise logging code paths without caring what gets logged.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// newCapturingLogger returns a *slog.Logger backed by a buffer, for tests
// that need to assert on what got logged (level, message, attributes).
func newCapturingLogger() (*slog.Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	return slog.New(slog.NewTextHandler(buf, nil)), buf
}

// newTestApp builds an *app with a real cookie store (so preventCSRF/
// currentSession/logout exercise real gorilla/sessions cookie handling)
// and fake-able cfg values for jwt.expiration_duration/refresh_interval
// and session.max_age — every value refreshIfNeeded/sessionTTL read.
// authRemote/sessionStore may be nil for tests that never reach a code
// path calling them.
func newTestApp(t *testing.T, authRemote resource.AuthRemote, sessionStore resource.SessionStore) *app {
	t.Helper()

	cfg := &config.Cfg{
		JWT: config.JWT{
			ExpirationDuration: "24h",
			RefreshInterval:    "1h",
		},
		Session: config.Session{
			MaxAge: 3600,
		},
	}

	return &app{
		cfg:          cfg,
		logger:       discardLogger(),
		authRemote:   authRemote,
		sessionStore: sessionStore,
		cookieStore:  sessions.NewCookieStore([]byte("test-secret-key-at-least-32-bytes")),
	}
}

// sessionCookieRequest builds a request carrying a valid signed session
// cookie whose keys.SessionID value is sessionID, as if the browser had
// already logged in — mirrors the same-named helper in
// handler/user_test.go.
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
	return r
}
