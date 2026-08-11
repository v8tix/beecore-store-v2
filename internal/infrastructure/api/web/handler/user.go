package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/sessions"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/core/port/resource"
	"github.com/v8tix/beecore-store-v2/internal/core/port/service"
	"github.com/v8tix/beecore-store-v2/internal/foundation/keys"
	"github.com/v8tix/beecore-store-v2/internal/foundation/web"
	"github.com/v8tix/beecore-store-v2/internal/request"
	"github.com/v8tix/beecore-store-v2/internal/response"
	"github.com/v8tix/beecore-store-v2/internal/validator"
)

// UserHandler holds the chi handler ported from the updateUserWizard
// section of cmd/web/handlers.go in the source repo. It calls
// port/service.User only for the actual profile update; loading and
// re-saving the session record around that call is still handler-level
// plumbing here, mirroring the source's currentSession/updateSession
// helpers, since their equivalent hasn't been ported as shared middleware
// yet (composition-root wiring, plan Task 19) — once it is, this handler
// can be thinned further to pick up whatever that middleware resolves
// instead of doing it itself.
type UserHandler struct {
	UserService service.User

	// Sessions/SessionCookieName resolve the browser's opaque session-ID
	// cookie, same as AuthHandler's identically-named fields.
	Sessions          *sessions.CookieStore
	SessionCookieName string

	// SessionStore loads/re-saves the server-side domain.Session record
	// the cookie's ID points at (app.sessions in the source repo).
	SessionStore resource.SessionStore

	// SessionTTL is the Redis session record's lifetime, same value used
	// by core/auth.Dependencies.SessionTTL — resolved once at startup
	// from config.Cfg.Session.MaxAge (sessionTTL() in the source repo).
	SessionTTL time.Duration

	Logger *slog.Logger
}

func NewUserHandler(
	userService service.User,
	sessionStore resource.SessionStore,
	cookieStore *sessions.CookieStore,
	sessionTTL time.Duration,
	logger *slog.Logger,
) *UserHandler {
	return &UserHandler{
		UserService:       userService,
		Sessions:          cookieStore,
		SessionCookieName: keys.Session,
		SessionStore:      sessionStore,
		SessionTTL:        sessionTTL,
		Logger:            logger,
	}
}

// currentSession resolves the cookie's opaque session ID and loads the
// server-side domain.Session it points at — a reduced version of
// currentSession in the source repo's cmd/web/helpers.go, without its
// access-token-refresh step: refreshIfNeeded's equivalent isn't wired up
// anywhere in this repo yet (composition-root wiring, plan Task 19, will
// add it once, and every handler — this one included — will pick it up
// for free through that middleware rather than duplicating the refresh
// call here).
func (h *UserHandler) currentSession(r *http.Request) (sessionID string, sess domain.Session, ok bool) {
	cookie, err := h.Sessions.Get(r, h.SessionCookieName)
	if err != nil {
		return "", domain.Session{}, false
	}

	v, exists := cookie.Values[keys.SessionID]
	if !exists {
		return "", domain.Session{}, false
	}

	sessionID, ok = v.(string)
	if !ok || sessionID == "" {
		return "", domain.Session{}, false
	}

	sess, err = h.SessionStore.Load(r.Context(), sessionID)
	if err != nil {
		return sessionID, domain.Session{}, false
	}

	return sessionID, sess, true
}

// UpdateWizard mirrors the updateUserWizard handler in the source repo.
func (h *UserHandler) UpdateWizard(w http.ResponseWriter, r *http.Request) {
	var form struct {
		IN          string              `form:"IN"`
		PhoneNumber string              `form:"PhoneNumber"`
		Birthday    string              `form:"Birthday"`
		Validator   validator.Validator `form:"-"`
	}

	sessionID, sess, ok := h.currentSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	switch r.Method {
	case http.MethodGet:
		// Mirrors the source handler's GET branch exactly, including its
		// quirk: the redirect check reads keys.SessionUserIN from the
		// browser cookie's own Values, not from domain.Session — and
		// nothing in the source repo ever sets that cookie key, so this
		// branch is dead in practice. Preserved as-is (1:1 port), not
		// fixed or dropped.
		if cookie, err := h.Sessions.Get(r, h.SessionCookieName); err == nil {
			if v, exists := cookie.Values[keys.SessionUserIN]; exists {
				if s, isStr := v.(string); isStr && s != "" {
					http.Redirect(w, r, "/user/create/address", http.StatusSeeOther)
					return
				}
			}
		}

		data := newTemplateData(r)
		data[web.Form] = form

		if err := response.Page(w, http.StatusOK, data, "pages/user-wizard-tmpl.html"); err != nil {
			serverError(h.Logger, w, r, err)
		}
		return

	case http.MethodPost:
		if err := request.DecodePostForm(r, &form); err != nil {
			badRequest(h.Logger, w, r, err)
			return
		}

		form.Validator.CheckField(len(form.IN) != 0, web.IN, web.MsgINRequired)
		form.Validator.CheckField(len(form.PhoneNumber) != 0, web.PhoneNumber, web.MsgPhoneNumberRequired)
		form.Validator.CheckField(len(form.Birthday) != 0, web.Birthday, web.MsgBirthdayRequired)

		if form.Validator.HasErrors() {
			data := newTemplateData(r)
			data[web.Form] = form

			if err := response.Page(w, http.StatusUnprocessableEntity, data, "pages/user-wizard-tmpl.html"); err != nil {
				serverError(h.Logger, w, r, err)
			}
			return
		}

		user := contextGetAuthenticatedUser(r)
		if user == nil {
			serverError(h.Logger, w, r, errAuthenticatedUserRequired)
			return
		}

		err := h.UserService.UpdateProfile(r.Context(), user.ID, user.FirstName, user.LastName, form.IN, form.Birthday, form.PhoneNumber)
		switch {
		case err == nil:
			sess.UserDNI = form.IN
			if err := h.SessionStore.Save(r.Context(), sessionID, sess, h.SessionTTL); err != nil {
				serverError(h.Logger, w, r, err)
				return
			}

		case errors.Is(err, domain.ErrUserNotFound):
			// Mirrors the source handler's 404 special-case: the
			// downstream user record is gone, so the session's DNI is
			// left unchanged (no SessionStore.Save call) — but, same as
			// the source's "break" out of its switch statement, the
			// request still proceeds to the redirect below rather than
			// erroring out.

		default:
			serverError(h.Logger, w, r, err)
			return
		}

		// The source handler's next step here — creating the user's
		// basket via CreateBasket and seeding the cookie's basket-ID
		// value before redirecting — is not ported: that depends on
		// resource.BasketRemote, which doesn't exist yet (Basket slice,
		// plan Task 13/14). The redirect itself is what actually drives
		// the wizard flow forward, so it's preserved directly; the
		// basket-ID cookie step is picked back up once the Basket slice
		// lands — same deferral AuthHandler.Login already documents for
		// its own basket-lookup step.
		http.Redirect(w, r, "/user/create/address", http.StatusSeeOther)
	}
}
