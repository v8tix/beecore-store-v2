package web

import (
	"net/http"

	"github.com/v8tix/beecore-store-v2/internal/foundation/keys"
	"github.com/v8tix/beecore-store-v2/internal/infrastructure/api/web/handler"
	"github.com/v8tix/beecore-store-v2/internal/response"
)

// home and logout mirror their same-named handlers in the source repo's
// cmd/web/handlers.go. Neither has a natural vertical-slice owner — every
// other cmd/web/handlers.go section moved into its own slice's
// infrastructure/api/web/handler/*.go — so both live directly on this
// composition root's app type instead, same precedent as
// beecore-admin-v2's own home.go.
//
// Unlike beecore-admin-v2's home (reached only once already authenticated),
// this app's "/" route sits behind requireAnonymousUser (see routes.go) —
// mirroring the source repo's routes.go exactly — so home itself needs no
// session/redirect logic of its own: an authenticated visitor never reaches
// it in the first place.
func (a *app) home(w http.ResponseWriter, r *http.Request) {
	data := handler.NewTemplateData(r)

	if err := response.Page(w, http.StatusOK, data, "pages/landing-tmpl.html"); err != nil {
		a.serverError(w, r, err)
	}
}

func (a *app) logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := a.cookieStore.Get(r, keys.Session)
	if err != nil {
		a.serverError(w, r, err)
		return
	}

	// Real server-side revocation: delete the Redis-held session record so
	// a stolen/cached cookie is rejected immediately, not just until the
	// cookie happens to get cleared client-side.
	if sessionID, ok := getSessionString(cookie, keys.SessionID); ok {
		if err := a.sessionStore.Delete(r.Context(), sessionID); err != nil {
			a.logger.Error("failed to delete session on logout", "error", err)
		}
	}

	cookie.Values = map[any]any{}
	cookie.Options.MaxAge = -1

	if err := cookie.Save(r, w); err != nil {
		a.serverError(w, r, err)
		return
	}

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
