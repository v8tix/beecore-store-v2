package web

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gorilla/sessions"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/foundation/keys"
)

// getSessionString mirrors application.getSessionString in the source
// repo's cmd/web/helpers.go.
func getSessionString(cookie *sessions.Session, key any) (string, bool) {
	v, ok := cookie.Values[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	if !ok || len(s) == 0 {
		return "", false
	}
	return s, true
}

// currentSession resolves the cookie's opaque session ID and loads the
// server-side domain.Session it points at, refreshing the access token
// first if it's close to expiry. Mirrors application.currentSession in the
// source repo's cmd/web/helpers.go — this is the one place in this repo
// that still does the full resolve+refresh dance; every handler's own
// session read (package handler's per-handler currentSession, used once
// this middleware has already run) is a reduced version that assumes the
// session it loads is already fresh.
//
// ok is false when there's no session cookie, or the session record is
// gone (expired naturally, removed by logout, or invalidated here because
// the backend explicitly rejected a refresh) — callers treat that exactly
// like "not logged in".
func (a *app) currentSession(r *http.Request) (cookie *sessions.Session, sessionID string, sess domain.Session, ok bool) {
	cookie, err := a.cookieStore.Get(r, keys.Session)
	if err != nil {
		return cookie, "", domain.Session{}, false
	}

	sessionID, ok = getSessionString(cookie, keys.SessionID)
	if !ok {
		return cookie, "", domain.Session{}, false
	}

	sess, err = a.sessionStore.Load(r.Context(), sessionID)
	if err != nil {
		return cookie, sessionID, domain.Session{}, false
	}

	refreshed, err := a.refreshIfNeeded(r.Context(), sessionID, sess)
	if err != nil {
		if errors.Is(err, domain.ErrSessionRejected) {
			// The backend explicitly rejected the refresh — revoked/
			// expired/mismatched refresh token, or a disabled user — so
			// the session is genuinely dead, not just stale. Remove it now
			// rather than let every subsequent request keep retrying a
			// refresh that will never succeed with a decaying (or
			// already-expired) access token in the meantime.
			if delErr := a.sessionStore.Delete(r.Context(), sessionID); delErr != nil {
				a.logger.Error("failed to delete rejected session", "error", delErr, "user_id", sess.UserID)
			}
			return cookie, "", domain.Session{}, false
		}

		a.logger.Error("session access-token refresh failed", "error", err, "user_id", sess.UserID)
		return cookie, sessionID, sess, true
	}

	return cookie, sessionID, refreshed, true
}

// refreshIfNeeded exchanges the session's access token for a new one once
// it's within jwt.refresh_interval of its own expiry. This runs
// synchronously rather than in the background (contrast
// AuthService.GetOrGenerateAuthenticationToken's refresh-ahead, which is
// hit concurrently by every request across the whole app and so needs a
// background goroutine + singleflight): a user session is driven by one
// browser tab at a time, so a single blocking call here adds at most one
// extra request, at most once per refresh_interval, only when needed.
// Mirrors application.refreshIfNeeded in the source repo's
// cmd/web/helpers.go.
func (a *app) refreshIfNeeded(reqCtx context.Context, sessionID string, sess domain.Session) (domain.Session, error) {
	interval, err := a.cfg.JWT.GetRefreshInterval()
	if err != nil {
		return sess, err
	}

	if !needsRefresh(time.Now(), sess.UserAccessExpiry, interval) {
		return sess, nil
	}

	adminToken, err := a.authRemote.GetAdminToken(reqCtx)
	if err != nil {
		return sess, err
	}

	accessToken, err := a.authRemote.RefreshUserToken(reqCtx, sess.UserID, sess.RefreshToken, adminToken)
	if err != nil {
		return sess, err
	}
	if accessToken == "" {
		return sess, nil
	}

	duration, err := a.cfg.JWT.GetDuration()
	if err != nil {
		return sess, err
	}

	sess.UserAccessToken = accessToken
	sess.UserAccessExpiry = time.Now().Add(duration)

	if err := a.sessionStore.Save(reqCtx, sessionID, sess, a.sessionTTL()); err != nil {
		return sess, err
	}

	return sess, nil
}

// needsRefresh reports whether an access token expiring at accessExpiry is
// close enough (within refreshInterval) to its own expiry, as of now, that
// it should be refreshed rather than used as-is. Mirrors the free function
// of the same name in the source repo's cmd/web/helpers.go.
func needsRefresh(now, accessExpiry time.Time, refreshInterval time.Duration) bool {
	return accessExpiry.Sub(now) <= refreshInterval
}

// sessionTTL mirrors application.sessionTTL in the source repo's
// cmd/web/helpers.go — resolved once here (used by preventCSRF's cookie
// MaxAge, and every setupHandlers call that needs the same value) rather
// than read from cfg afresh at every call site.
func (a *app) sessionTTL() time.Duration {
	return sessionTTLFromCfg(a.cfg)
}
