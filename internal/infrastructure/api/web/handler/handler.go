// Package handler holds chi handlers, one type per vertical slice, each
// calling its slice's core/port/service interface only — never kawa, never
// a repository, never another service's internals directly. This file
// holds the pieces every handler in the package needs: template-data
// assembly, the authenticated-user request context key, and the
// serverError/badRequest helpers, all ported from cmd/web/{context,
// helpers,errors}.go in the source repo.
package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/justinas/nosurf"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/version"
)

// errAuthenticatedUserRequired mirrors ErrAuthenticatedUserRequired in the
// source repo's cmd/web/context.go — returned when a handler that assumes
// an authenticated user in context (populated by the composition root's
// authenticate middleware, which doesn't exist in this repo yet) finds
// none. Shared by every handler in this package that has that same
// assumption (UserHandler, AddressHandler), mirroring how each of their
// source counterparts referenced the same package-level var.
var errAuthenticatedUserRequired = errors.New("authenticated user is required")

type contextKey string

const authenticatedUserContextKey = contextKey("authenticatedUser")

// contextSetAuthenticatedUser mirrors contextSetAuthenticatedUser in the
// source repo's cmd/web/context.go, holding a domain.User instead of the
// downstream users.User wire type.
func contextSetAuthenticatedUser(r *http.Request, user *domain.User) *http.Request {
	c := context.WithValue(r.Context(), authenticatedUserContextKey, user)
	return r.WithContext(c)
}

func contextGetAuthenticatedUser(r *http.Request) *domain.User {
	user, ok := r.Context().Value(authenticatedUserContextKey).(*domain.User)
	if !ok {
		return nil
	}

	return user
}

// newTemplateData mirrors application.newTemplateData in the source
// repo's cmd/web/helpers.go.
func newTemplateData(r *http.Request) map[string]any {
	return map[string]any{
		"AuthenticatedUser": contextGetAuthenticatedUser(r),
		"CSRFToken":         nosurf.Token(r),
		"Version":           version.Get(),
	}
}

// SetAuthenticatedUser, GetAuthenticatedUser and NewTemplateData are this
// package's exported entry points for the composition root
// (infrastructure/api/web, plan Task 19), which has no vertical-slice
// package of its own: its authenticate/requireAuthenticatedUser middleware
// populates the request context via SetAuthenticatedUser (mirroring
// contextSetAuthenticatedUser in the source repo's cmd/web/context.go, run
// once per request rather than per-handler), and its home/logout handlers
// (no natural vertical-slice owner — see that file's doc comment) call
// NewTemplateData the same way every handler in this package already does
// via the unexported newTemplateData. Every handler in this package
// continues to use the unexported contextGetAuthenticatedUser/
// contextSetAuthenticatedUser/newTemplateData directly — these are thin
// aliases for the one caller outside this package, matching the precedent
// set by beecore-admin-v2's own handler package.
func SetAuthenticatedUser(r *http.Request, user *domain.User) *http.Request {
	return contextSetAuthenticatedUser(r, user)
}

func GetAuthenticatedUser(r *http.Request) *domain.User {
	return contextGetAuthenticatedUser(r)
}

func NewTemplateData(r *http.Request) map[string]any {
	return newTemplateData(r)
}

// logError, serverError and badRequest mirror their same-named methods on
// application in the source repo's cmd/web/errors.go. Unlike the source's
// serverErrorFromHTTP, there is no kawa-aware variant here — every error
// a handler in this package can see is either a domain.Err* sentinel
// (handled explicitly by the caller before falling through to
// serverError) or an already-translated plain error from a
// port/service.Auth method, never a kawa.ErrInvalidHTTPStatus.
func logError(logger *slog.Logger, r *http.Request, err error) {
	logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
}

func serverError(logger *slog.Logger, w http.ResponseWriter, r *http.Request, err error) {
	logError(logger, r, err)

	message := "The server encountered a problem and could not process your request"
	http.Error(w, fmt.Sprintf("%s: %s", message, err.Error()), http.StatusInternalServerError)
}

func badRequest(logger *slog.Logger, w http.ResponseWriter, r *http.Request, err error) {
	logError(logger, r, err)

	http.Error(w, err.Error(), http.StatusBadRequest)
}
