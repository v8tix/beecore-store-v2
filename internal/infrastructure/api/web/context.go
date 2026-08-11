// Package web is the composition root for the storefront web app: the chi
// router, middleware, and every vertical slice's handler, wired together
// from *config.Cfg. Ported from cmd/web/{main,server,routes,middleware,
// context,errors,helpers}.go in the source repo, split the same way
// beecore-admin-v2's composition root splits its own: setupResources
// (building each vertical slice's outbound adapter from cfg) →
// setupServices (each slice's use-case) → setupHandlers (each slice's chi
// handler) → routes.go/middleware.go wiring them together and starting the
// server (server.go).
//
// This file mirrors cmd/web/context.go in the source repo:
// contextSetAuthenticatedUser/contextGetAuthenticatedUser under the same
// short names the source repo used. The context key itself lives in
// package handler (since every handler there already reads it via
// handler.GetAuthenticatedUser) — these are thin aliases so middleware.go
// and home.go in this package can use the source repo's original names
// rather than handler.SetAuthenticatedUser/handler.GetAuthenticatedUser
// directly.
package web

import (
	"net/http"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/infrastructure/api/web/handler"
)

func contextSetAuthenticatedUser(r *http.Request, user *domain.User) *http.Request {
	return handler.SetAuthenticatedUser(r, user)
}

func contextGetAuthenticatedUser(r *http.Request) *domain.User {
	return handler.GetAuthenticatedUser(r)
}
