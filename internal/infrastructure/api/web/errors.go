package web

import (
	"fmt"
	"net/http"
)

// logError, serverError and notFound mirror their same-named methods on
// application in the source repo's cmd/web/errors.go. badRequest isn't
// ported here — nothing at this layer decodes a request body; every
// badRequest call in this app happens inside package handler, which has
// its own copy. There's likewise no serverErrorFromHTTP variant: every
// error this package's middleware can see from a port/resource.AuthRemote
// call is already a domain.Err* sentinel or a plain wrapped error — kawa
// never leaks this far up (see resource.AuthRemote's doc comment).
func (a *app) logError(r *http.Request, err error) {
	a.logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
}

func (a *app) serverError(w http.ResponseWriter, r *http.Request, err error) {
	a.logError(r, err)

	message := "The server encountered a problem and could not process your request"
	http.Error(w, fmt.Sprintf("%s: %s", message, err.Error()), http.StatusInternalServerError)
}

func (a *app) notFound(w http.ResponseWriter, r *http.Request) {
	a.logger.Warn("not found", "method", r.Method, "uri", r.URL.RequestURI())

	message := "The requested resource could not be found"
	http.Error(w, message, http.StatusNotFound)
}
