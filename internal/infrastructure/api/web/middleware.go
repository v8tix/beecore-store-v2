package web

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/justinas/nosurf"
)

// recoverPanic mirrors application.recoverPanic in the source repo's
// cmd/web/middleware.go.
func (a *app) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				a.logger.Error("panic recovered",
					"error", err,
					"method", r.Method,
					"url", r.URL.String(),
					"remote_addr", r.RemoteAddr,
					"user_agent", r.UserAgent(),
					"stack", string(debug.Stack()),
				)

				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// securityHeaders mirrors application.securityHeaders in the source repo's
// cmd/web/middleware.go.
func (a *app) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Referrer-Policy", "origin-when-cross-origin")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "deny")
		w.Header().Set("X-XSS-Protection", "0")
		next.ServeHTTP(w, r)
	})
}

// preventCSRF mirrors application.preventCSRF in the source repo's
// cmd/web/middleware.go.
func (a *app) preventCSRF(next http.Handler) http.Handler {
	csrfHandler := nosurf.New(next)
	csrfHandler.SetBaseCookie(http.Cookie{
		HttpOnly: true,
		Path:     "/",
		// Tracks session.max_age — the CSRF cookie shouldn't outlive the
		// session it protects.
		MaxAge:   int(a.sessionTTL().Seconds()),
		SameSite: http.SameSiteLaxMode,
		Secure:   true,
	})
	return csrfHandler
}

// authenticate middleware checks if a user is logged in. Session
// resolution (including near-expiry access-token refresh) happens once
// here, via currentSession; requireAuthenticatedUser below only
// re-resolves it if this didn't already populate the request context.
// Mirrors application.authenticate in the source repo's
// cmd/web/middleware.go.
func (a *app) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _, sess, ok := a.currentSession(r)
		if ok {
			authenticatedUser := contextGetAuthenticatedUser(r)

			if authenticatedUser != nil {
				next.ServeHTTP(w, r)
				return
			}

			adminToken, err := a.authRemote.GetAdminToken(r.Context())
			if err != nil {
				a.serverError(w, r, err)
				return
			}

			user, err := a.authRemote.FindUserByID(r.Context(), sess.UserID, adminToken)
			if err != nil {
				a.serverError(w, r, err)
				return
			}

			// Mirrors the source middleware's res.Body != nil check: a
			// nil-body-but-no-error response leaves the request
			// unauthenticated rather than injecting a zero-value user.
			if user.ID != "" {
				r = contextSetAuthenticatedUser(r, &user)
			}
		}

		next.ServeHTTP(w, r)
	})
}

// requireAuthenticatedUser requires a user to be logged in. Mirrors
// application.requireAuthenticatedUser in the source repo's
// cmd/web/middleware.go.
func (a *app) requireAuthenticatedUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authenticatedUser := contextGetAuthenticatedUser(r)
		if authenticatedUser == nil {
			_, _, sess, ok := a.currentSession(r)
			if ok {
				adminToken, err := a.authRemote.GetAdminToken(r.Context())
				if err != nil {
					a.serverError(w, r, err)
					return
				}

				user, err := a.authRemote.FindUserByID(r.Context(), sess.UserID, adminToken)
				if err != nil {
					a.serverError(w, r, err)
					return
				}

				if user.ID != "" {
					r = contextSetAuthenticatedUser(r, &user)
					w.Header().Add("Cache-Control", "no-store")
					next.ServeHTTP(w, r)
					return
				}

				http.Redirect(w, r, "/", http.StatusSeeOther)
				return
			}

			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		w.Header().Add("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// requireAnonymousUser requires a user to NOT be logged in. Mirrors
// application.requireAnonymousUser in the source repo's
// cmd/web/middleware.go, including its own doc comment's reasoning: "/" is
// itself guarded by requireAnonymousUser, so redirecting there would loop
// forever — send logged-in users to the product search page instead.
func (a *app) requireAnonymousUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if contextGetAuthenticatedUser(r) != nil {
			http.Redirect(w, r, "/search", http.StatusSeeOther)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// metrics middleware for tracking request durations. Mirrors
// application.metrics in the source repo's cmd/web/middleware.go.
func (a *app) metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(rw, r)

		duration := time.Since(start)
		attrs := []any{
			slog.String("path", r.URL.Path),
			slog.String("method", r.Method),
			slog.Int("status", rw.statusCode),
			slog.Int64("duration_ms", duration.Milliseconds()),
		}

		switch {
		case rw.statusCode >= http.StatusInternalServerError:
			a.logger.Error("request metrics", attrs...)
		case rw.statusCode >= http.StatusBadRequest:
			a.logger.Warn("request metrics", attrs...)
		default:
			a.logger.Info("request metrics", attrs...)
		}
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
