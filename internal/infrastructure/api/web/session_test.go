package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/sessions"
	"github.com/stretchr/testify/mock"

	"github.com/v8tix/beecore-eda/config"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/core/port/resource/mocks"
	"github.com/v8tix/beecore-store-v2/internal/foundation/keys"
)

func TestGetSessionString(t *testing.T) {
	tests := []struct {
		name    string
		values  map[any]any
		key     any
		wantVal string
		wantOK  bool
	}{
		{"missing key", map[any]any{}, keys.SessionID, "", false},
		{"wrong type", map[any]any{keys.SessionID: 42}, keys.SessionID, "", false},
		{"empty string is treated as absent", map[any]any{keys.SessionID: ""}, keys.SessionID, "", false},
		{"valid string", map[any]any{keys.SessionID: "abc123"}, keys.SessionID, "abc123", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cookie := &sessions.Session{Values: tt.values}
			gotVal, gotOK := getSessionString(cookie, tt.key)
			if gotVal != tt.wantVal || gotOK != tt.wantOK {
				t.Fatalf("got (%q, %v), want (%q, %v)", gotVal, gotOK, tt.wantVal, tt.wantOK)
			}
		})
	}
}

func TestNeedsRefresh(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	interval := 24 * time.Hour

	tests := []struct {
		name         string
		accessExpiry time.Time
		want         bool
	}{
		{"well within validity, no refresh", now.Add(48 * time.Hour), false},
		{"just outside the refresh window, no refresh", now.Add(interval + time.Second), false},
		{"exactly at the refresh window boundary, refresh", now.Add(interval), true},
		{"inside the refresh window, refresh", now.Add(time.Hour), true},
		{"already expired, refresh", now.Add(-time.Hour), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := needsRefresh(now, tt.accessExpiry, interval)
			if got != tt.want {
				t.Fatalf("needsRefresh(now, %v, %v) = %v, want %v", tt.accessExpiry, interval, got, tt.want)
			}
		})
	}
}

func TestSessionTTLFromCfg(t *testing.T) {
	t.Run("unset or non-positive max age defaults to 7 days", func(t *testing.T) {
		if got := sessionTTLFromCfg(&config.Cfg{}); got != 7*24*time.Hour {
			t.Fatalf("got %v, want %v", got, 7*24*time.Hour)
		}
		if got := sessionTTLFromCfg(&config.Cfg{Session: config.Session{MaxAge: -1}}); got != 7*24*time.Hour {
			t.Fatalf("got %v, want %v", got, 7*24*time.Hour)
		}
	})

	t.Run("positive max age is honored", func(t *testing.T) {
		got := sessionTTLFromCfg(&config.Cfg{Session: config.Session{MaxAge: 60}})
		if want := 60 * time.Second; got != want {
			t.Fatalf("got %v, want %v", got, want)
		}
	})
}

func TestApp_SessionTTL(t *testing.T) {
	a := newTestApp(t, nil, nil)
	if got := a.sessionTTL(); got != 3600*time.Second {
		t.Fatalf("got %v, want %v", got, 3600*time.Second)
	}
}

func TestApp_RefreshIfNeeded(t *testing.T) {
	baseSess := domain.Session{UserID: "u1", RefreshToken: "rt1"}

	t.Run("invalid refresh interval config returns an error", func(t *testing.T) {
		a := newTestApp(t, mocks.NewAuthRemote(t), mocks.NewSessionStore(t))
		a.cfg.JWT.RefreshInterval = "not-a-duration"

		_, err := a.refreshIfNeeded(context.Background(), "sess-1", baseSess)
		if err == nil {
			t.Fatal("want an error for an unparsable refresh interval")
		}
	})

	t.Run("token not close to expiry: no refresh attempted", func(t *testing.T) {
		authRemote := mocks.NewAuthRemote(t) // no .On calls — any call fails the test
		a := newTestApp(t, authRemote, mocks.NewSessionStore(t))
		sess := baseSess
		sess.UserAccessExpiry = time.Now().Add(48 * time.Hour)

		got, err := a.refreshIfNeeded(context.Background(), "sess-1", sess)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.UserAccessToken != "" {
			t.Fatalf("got token %q, want the session left unchanged", got.UserAccessToken)
		}
	})

	t.Run("GetAdminToken failure propagates", func(t *testing.T) {
		authRemote := mocks.NewAuthRemote(t)
		authRemote.On("GetAdminToken", mock.Anything).Return("", errors.New("boom"))

		a := newTestApp(t, authRemote, mocks.NewSessionStore(t))
		sess := baseSess
		sess.UserAccessExpiry = time.Now().Add(-time.Hour)

		if _, err := a.refreshIfNeeded(context.Background(), "sess-1", sess); err == nil {
			t.Fatal("want an error")
		}
	})

	t.Run("RefreshUserToken rejection propagates domain.ErrSessionRejected", func(t *testing.T) {
		authRemote := mocks.NewAuthRemote(t)
		authRemote.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)
		authRemote.On("RefreshUserToken", mock.Anything, "u1", "rt1", "admin-tok").Return("", domain.ErrSessionRejected)

		a := newTestApp(t, authRemote, mocks.NewSessionStore(t))
		sess := baseSess
		sess.UserAccessExpiry = time.Now().Add(-time.Hour)

		_, err := a.refreshIfNeeded(context.Background(), "sess-1", sess)
		if !errors.Is(err, domain.ErrSessionRejected) {
			t.Fatalf("got %v, want domain.ErrSessionRejected", err)
		}
	})

	t.Run("empty access token from RefreshUserToken leaves the session unchanged", func(t *testing.T) {
		authRemote := mocks.NewAuthRemote(t)
		authRemote.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)
		authRemote.On("RefreshUserToken", mock.Anything, "u1", "rt1", "admin-tok").Return("", nil)
		sessionStore := mocks.NewSessionStore(t) // no Save call expected

		a := newTestApp(t, authRemote, sessionStore)
		sess := baseSess
		sess.UserAccessExpiry = time.Now().Add(-time.Hour)

		got, err := a.refreshIfNeeded(context.Background(), "sess-1", sess)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.UserAccessToken != "" {
			t.Fatalf("got token %q, want the session left unchanged", got.UserAccessToken)
		}
	})

	t.Run("invalid JWT duration config propagates after a successful token refresh", func(t *testing.T) {
		authRemote := mocks.NewAuthRemote(t)
		authRemote.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)
		authRemote.On("RefreshUserToken", mock.Anything, "u1", "rt1", "admin-tok").Return("new-tok", nil)

		a := newTestApp(t, authRemote, mocks.NewSessionStore(t))
		a.cfg.JWT.ExpirationDuration = "bogus"
		sess := baseSess
		sess.UserAccessExpiry = time.Now().Add(-time.Hour)

		if _, err := a.refreshIfNeeded(context.Background(), "sess-1", sess); err == nil {
			t.Fatal("want an error")
		}
	})

	t.Run("sessionStore.Save failure propagates", func(t *testing.T) {
		authRemote := mocks.NewAuthRemote(t)
		authRemote.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)
		authRemote.On("RefreshUserToken", mock.Anything, "u1", "rt1", "admin-tok").Return("new-tok", nil)
		sessionStore := mocks.NewSessionStore(t)
		sessionStore.On("Save", mock.Anything, "sess-1", mock.Anything, mock.Anything).Return(errors.New("redis down"))

		a := newTestApp(t, authRemote, sessionStore)
		sess := baseSess
		sess.UserAccessExpiry = time.Now().Add(-time.Hour)

		if _, err := a.refreshIfNeeded(context.Background(), "sess-1", sess); err == nil {
			t.Fatal("want an error")
		}
	})

	t.Run("successful refresh updates the access token and expiry and persists", func(t *testing.T) {
		authRemote := mocks.NewAuthRemote(t)
		authRemote.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)
		authRemote.On("RefreshUserToken", mock.Anything, "u1", "rt1", "admin-tok").Return("new-tok", nil)
		sessionStore := mocks.NewSessionStore(t)
		sessionStore.On("Save", mock.Anything, "sess-1", mock.MatchedBy(func(s domain.Session) bool {
			return s.UserAccessToken == "new-tok"
		}), mock.Anything).Return(nil)

		a := newTestApp(t, authRemote, sessionStore)
		sess := baseSess
		sess.UserAccessExpiry = time.Now().Add(-time.Hour)

		got, err := a.refreshIfNeeded(context.Background(), "sess-1", sess)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.UserAccessToken != "new-tok" {
			t.Fatalf("got token %q, want %q", got.UserAccessToken, "new-tok")
		}
		if !got.UserAccessExpiry.After(time.Now()) {
			t.Fatalf("got expiry %v, want it in the future", got.UserAccessExpiry)
		}
	})
}

func TestApp_CurrentSession(t *testing.T) {
	t.Run("no session cookie", func(t *testing.T) {
		a := newTestApp(t, mocks.NewAuthRemote(t), mocks.NewSessionStore(t))

		_, _, _, ok := a.currentSession(httptest.NewRequest(http.MethodGet, "/", nil))
		if ok {
			t.Fatal("want ok=false when there is no session cookie")
		}
	})

	t.Run("undecodable session cookie", func(t *testing.T) {
		a := newTestApp(t, mocks.NewAuthRemote(t), mocks.NewSessionStore(t))

		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.AddCookie(&http.Cookie{Name: keys.Session, Value: "not-a-valid-signed-cookie"})

		_, _, _, ok := a.currentSession(r)
		if ok {
			t.Fatal("want ok=false when the session cookie can't be decoded")
		}
	})

	t.Run("session load failure", func(t *testing.T) {
		sessionStore := mocks.NewSessionStore(t)
		sessionStore.On("Load", mock.Anything, "sess-1").Return(domain.Session{}, errors.New("not found"))

		a := newTestApp(t, mocks.NewAuthRemote(t), sessionStore)
		r := sessionCookieRequest(t, a.cookieStore, http.MethodGet, "/", nil, "sess-1")

		_, _, _, ok := a.currentSession(r)
		if ok {
			t.Fatal("want ok=false when the session record can't be loaded")
		}
	})

	t.Run("valid session, no refresh needed", func(t *testing.T) {
		sess := domain.Session{UserID: "u1", UserAccessExpiry: time.Now().Add(48 * time.Hour)}
		sessionStore := mocks.NewSessionStore(t)
		sessionStore.On("Load", mock.Anything, "sess-1").Return(sess, nil)

		a := newTestApp(t, mocks.NewAuthRemote(t), sessionStore)
		r := sessionCookieRequest(t, a.cookieStore, http.MethodGet, "/", nil, "sess-1")

		_, sessionID, gotSess, ok := a.currentSession(r)
		if !ok {
			t.Fatal("want ok=true for a valid session")
		}
		if sessionID != "sess-1" {
			t.Fatalf("got session ID %q, want %q", sessionID, "sess-1")
		}
		if gotSess.UserID != "u1" {
			t.Fatalf("got user ID %q, want %q", gotSess.UserID, "u1")
		}
	})

	t.Run("refresh rejected deletes the session and reports not-logged-in", func(t *testing.T) {
		sess := domain.Session{UserID: "u1", RefreshToken: "rt1", UserAccessExpiry: time.Now().Add(-time.Hour)}
		sessionStore := mocks.NewSessionStore(t)
		sessionStore.On("Load", mock.Anything, "sess-1").Return(sess, nil)
		sessionStore.On("Delete", mock.Anything, "sess-1").Return(nil)
		authRemote := mocks.NewAuthRemote(t)
		authRemote.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)
		authRemote.On("RefreshUserToken", mock.Anything, "u1", "rt1", "admin-tok").Return("", domain.ErrSessionRejected)

		a := newTestApp(t, authRemote, sessionStore)
		r := sessionCookieRequest(t, a.cookieStore, http.MethodGet, "/", nil, "sess-1")

		_, _, _, ok := a.currentSession(r)
		if ok {
			t.Fatal("want ok=false once the backend rejects the refresh")
		}
		sessionStore.AssertCalled(t, "Delete", mock.Anything, "sess-1")
	})

	t.Run("refresh rejected but session delete also fails: still reports not-logged-in", func(t *testing.T) {
		sess := domain.Session{UserID: "u1", RefreshToken: "rt1", UserAccessExpiry: time.Now().Add(-time.Hour)}
		sessionStore := mocks.NewSessionStore(t)
		sessionStore.On("Load", mock.Anything, "sess-1").Return(sess, nil)
		sessionStore.On("Delete", mock.Anything, "sess-1").Return(errors.New("redis down"))
		authRemote := mocks.NewAuthRemote(t)
		authRemote.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)
		authRemote.On("RefreshUserToken", mock.Anything, "u1", "rt1", "admin-tok").Return("", domain.ErrSessionRejected)

		logger, buf := newCapturingLogger()
		a := newTestApp(t, authRemote, sessionStore)
		a.logger = logger
		r := sessionCookieRequest(t, a.cookieStore, http.MethodGet, "/", nil, "sess-1")

		_, _, _, ok := a.currentSession(r)
		if ok {
			t.Fatal("want ok=false even when the delete itself fails")
		}
		if !strings.Contains(buf.String(), "failed to delete rejected session") {
			t.Fatalf("got log output %q, want it to mention the failed delete", buf.String())
		}
	})

	t.Run("transient refresh failure keeps the stale session usable", func(t *testing.T) {
		sess := domain.Session{UserID: "u1", RefreshToken: "rt1", UserAccessExpiry: time.Now().Add(-time.Hour)}
		sessionStore := mocks.NewSessionStore(t)
		sessionStore.On("Load", mock.Anything, "sess-1").Return(sess, nil)
		authRemote := mocks.NewAuthRemote(t)
		authRemote.On("GetAdminToken", mock.Anything).Return("", errors.New("network blip"))

		a := newTestApp(t, authRemote, sessionStore)
		r := sessionCookieRequest(t, a.cookieStore, http.MethodGet, "/", nil, "sess-1")

		_, sessionID, gotSess, ok := a.currentSession(r)
		if !ok {
			t.Fatal("want ok=true — a transient refresh failure keeps the session usable")
		}
		if sessionID != "sess-1" || gotSess.UserID != "u1" {
			t.Fatalf("got (%q, %+v), want (%q, user u1)", sessionID, gotSess, "sess-1")
		}
	})
}
