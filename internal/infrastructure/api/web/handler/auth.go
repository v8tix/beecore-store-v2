package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/sessions"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/core/port/service"
	"github.com/v8tix/beecore-store-v2/internal/foundation/keys"
	"github.com/v8tix/beecore-store-v2/internal/foundation/web"
	"github.com/v8tix/beecore-store-v2/internal/password"
	"github.com/v8tix/beecore-store-v2/internal/request"
	"github.com/v8tix/beecore-store-v2/internal/response"
	"github.com/v8tix/beecore-store-v2/internal/validator"
)

// AuthHandler holds the chi handlers ported from the login/signup/
// activate/forgottenPassword/passwordReset sections of cmd/web/handlers.go
// in the source repo. It calls port/service.Auth (and, for Login's
// basket-quantity-cookie step, port/service.Basket) only — every downstream
// call and business decision those handlers used to make directly (via
// kawa, through the shared BaseRepositoryImpl) now lives in core/auth's
// and core/basket's use-cases; this type keeps only what's still
// handler-appropriate: request decode/validate, the browser session
// cookie, redirects, and template rendering.
type AuthHandler struct {
	AuthService service.Auth

	// BasketService resolves Login's basket-quantity-cookie step — see
	// its doc comment. Not needed by Signup/Activate/ForgottenPassword/
	// PasswordReset, which never touch a basket.
	BasketService service.Basket

	// Sessions is the gorilla cookie store holding the browser's opaque
	// session-ID cookie (app.Session.SessionStore in the source repo,
	// built by config.Cfg.Session.initSessionStore — composition-root
	// wiring, not this package's concern). SessionCookieName is the
	// cookie's name (keys.Session's value, "session").
	Sessions          *sessions.CookieStore
	SessionCookieName string
	Logger            *slog.Logger
}

func NewAuthHandler(authService service.Auth, basketService service.Basket, sessionStore *sessions.CookieStore, logger *slog.Logger) *AuthHandler {
	return &AuthHandler{
		AuthService:       authService,
		BasketService:     basketService,
		Sessions:          sessionStore,
		SessionCookieName: keys.Session,
		Logger:            logger,
	}
}

// Signup mirrors the signup handler in the source repo.
func (h *AuthHandler) Signup(w http.ResponseWriter, r *http.Request) {
	var form struct {
		FirstName       string              `form:"FirstName"`
		LastName        string              `form:"LastName"`
		Email           string              `form:"Email"`
		Password        string              `form:"Password"`
		ConfirmPassword string              `form:"ConfirmPassword"`
		Validator       validator.Validator `form:"-"`
	}

	switch r.Method {
	case http.MethodGet:
		data := newTemplateData(r)
		data[web.Form] = form

		if err := response.Page(w, http.StatusOK, data, "pages/signup-tmpl-1.html"); err != nil {
			serverError(h.Logger, w, r, err)
		}

	case http.MethodPost:
		if err := request.DecodePostForm(r, &form); err != nil {
			badRequest(h.Logger, w, r, err)
			return
		}

		form.Validator.CheckField(len(form.FirstName) != 0, web.FirstName, web.MsgFirstNameRequired)
		form.Validator.CheckField(len(form.LastName) != 0, web.LastName, web.MsgLastNameRequired)
		form.Validator.CheckField(len(form.Email) != 0, web.Email, web.MsgEmailRequired)
		form.Validator.CheckField(validator.Matches(form.Email, validator.RgxEmail), web.Email, web.MsgInvalidEmail)

		form.Validator.CheckField(len(form.Password) != 0, web.Password, web.MsgPasswdRequired)
		form.Validator.CheckField(len(form.Password) >= 8, web.Password, web.MsgTooShortPasswd)
		form.Validator.CheckField(len(form.Password) <= 72, web.Password, web.MsgTooLongPasswd)
		form.Validator.CheckField(validator.NotIn(form.Password, password.CommonPasswords...), web.Password, web.MsgTooCommonPasswd)

		form.Validator.CheckField(len(form.ConfirmPassword) != 0, web.ConfirmPassword, web.MsgPasswdRequired)
		form.Validator.CheckField(len(form.ConfirmPassword) >= 8, web.ConfirmPassword, web.MsgTooShortPasswd)
		form.Validator.CheckField(len(form.ConfirmPassword) <= 72, web.ConfirmPassword, web.MsgTooLongPasswd)
		form.Validator.CheckField(validator.NotIn(form.ConfirmPassword, password.CommonPasswords...), web.ConfirmPassword, web.MsgTooCommonPasswd)

		form.Validator.CheckField(validator.AreEqual(form.Password, form.ConfirmPassword), web.Password, web.MsgConfirmPasswdNotMatch)
		form.Validator.CheckField(validator.AreEqual(form.Password, form.ConfirmPassword), web.ConfirmPassword, web.MsgConfirmPasswdNotMatch)

		if form.Validator.HasErrors() {
			data := newTemplateData(r)
			data[web.Form] = form

			if err := response.Page(w, http.StatusUnprocessableEntity, data, "pages/signup-tmpl-1.html"); err != nil {
				serverError(h.Logger, w, r, err)
			}
			return
		}

		userID, err := h.AuthService.Signup(r.Context(), form.FirstName, form.LastName, form.Email, form.Password, form.ConfirmPassword)
		if err != nil {
			if errors.Is(err, domain.ErrEmailInUse) {
				form.Validator.CheckField(false, web.Email, web.MsgEmailInUse)

				data := newTemplateData(r)
				data[web.Form] = form

				if err := response.Page(w, http.StatusUnprocessableEntity, data, "pages/signup-tmpl-1.html"); err != nil {
					serverError(h.Logger, w, r, err)
				}
				return
			}

			serverError(h.Logger, w, r, err)
			return
		}

		http.Redirect(w, r, fmt.Sprintf("/activate/%s", userID), http.StatusSeeOther)
	}
}

// Activate mirrors the activate handler in the source repo.
func (h *AuthHandler) Activate(w http.ResponseWriter, r *http.Request) {
	var form struct {
		ActivationCode string              `form:"ActivationCode"`
		Validator      validator.Validator `form:"-"`
	}

	userID := chi.URLParam(r, "user_id")
	if len(userID) == 0 {
		serverError(h.Logger, w, r, errors.New("missing or invalid user_id param"))
		return
	}

	switch r.Method {
	case http.MethodGet:
		data := newTemplateData(r)
		data[web.Form] = form
		data["userID"] = userID

		if err := response.Page(w, http.StatusOK, data, "pages/activate-tmpl.html"); err != nil {
			serverError(h.Logger, w, r, err)
		}
		return

	case http.MethodPost:
		if err := request.DecodePostForm(r, &form); err != nil {
			badRequest(h.Logger, w, r, err)
			return
		}

		form.Validator.CheckField(len(form.ActivationCode) != 0, web.ActivationCode, web.MsgInvalidActivationCode)

		if form.Validator.HasErrors() {
			data := newTemplateData(r)
			data[web.Form] = form
			data["userID"] = userID

			if err := response.Page(w, http.StatusUnprocessableEntity, data, "pages/activate-tmpl.html"); err != nil {
				serverError(h.Logger, w, r, err)
			}
			return
		}

		err := h.AuthService.Activate(r.Context(), userID, form.ActivationCode)
		if err == nil {
			http.Redirect(w, r, "/account/activated", http.StatusSeeOther)
			return
		}

		if errors.Is(err, domain.ErrInvalidActivationToken) {
			form.Validator.CheckField(false, web.ActivationCode, web.MsgInvalidActivationCode)

			data := newTemplateData(r)
			data[web.Form] = form
			data["userID"] = userID

			if err := response.Page(w, http.StatusUnprocessableEntity, data, "pages/activate-tmpl.html"); err != nil {
				serverError(h.Logger, w, r, err)
			}
			return
		}

		serverError(h.Logger, w, r, err)
	}
}

// Login mirrors the login handler in the source repo, including its two
// basket-quantity-cookie steps — deferred until the Basket slice existed
// (plan Task 13/14; see core/auth.Login's doc comment for the deferral this
// resolves), now wired in via BasketService:
//
//   - Right after a successful AuthService.Login, BasketService.FindByUserID
//     looks up the user's currently open basket and seeds the cookie's
//     keys.SessionBasketID value — errors are silently ignored here,
//     mirroring the source handler's own swallowed FindBasketByUserID
//     error (a user simply doesn't have a basket cookie yet in that case).
//   - Once fully onboarded (DNI and shipping address both present, the only
//     path that reaches the final /search redirect), the cookie's basket ID
//     must exist by then (a real error otherwise — mirrors the source
//     handler's keys.ErrSessionBasketIDRequired check) — BasketService.FindByID
//     loads its contents to recompute keys.SessionBasketItems via
//     sumQuantities before the redirect.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var form struct {
		Email     string              `form:"Email"`
		Password  string              `form:"Password"`
		Validator validator.Validator `form:"-"`
	}

	switch r.Method {
	case http.MethodGet:
		data := newTemplateData(r)
		data[web.Form] = form

		if err := response.Page(w, http.StatusOK, data, "pages/login-tmpl-1.html"); err != nil {
			serverError(h.Logger, w, r, err)
		}

	case http.MethodPost:
		if err := request.DecodePostForm(r, &form); err != nil {
			badRequest(h.Logger, w, r, err)
			return
		}

		form.Validator.CheckField(len(form.Email) != 0, web.Email, web.MsgEmailRequired)
		form.Validator.CheckField(len(form.Password) != 0, web.Password, web.MsgPasswdRequired)

		if form.Validator.HasErrors() {
			data := newTemplateData(r)
			data[web.Form] = form

			if err := response.Page(w, http.StatusUnprocessableEntity, data, "pages/login-tmpl-1.html"); err != nil {
				serverError(h.Logger, w, r, err)
			}
			return
		}

		cookie, err := h.Sessions.Get(r, h.SessionCookieName)
		if err != nil {
			serverError(h.Logger, w, r, err)
			return
		}

		sessionID, sess, user, err := h.AuthService.Login(r.Context(), form.Email, form.Password)
		if err != nil {
			switch {
			case errors.Is(err, domain.ErrInvalidCredentials):
				form.Validator.CheckField(false, web.Password, web.MsgIncorrectPasswd)
			case errors.Is(err, domain.ErrUserNotFound):
				form.Validator.CheckField(false, web.Email, web.MsgUserNotFound)
			}

			if form.Validator.HasErrors() {
				data := newTemplateData(r)
				data[web.Form] = form

				if err := response.Page(w, http.StatusUnprocessableEntity, data, "pages/login-tmpl-1.html"); err != nil {
					serverError(h.Logger, w, r, err)
				}
				return
			}

			serverError(h.Logger, w, r, err)
			return
		}

		// Mirrors the source handler's early FindBasketByUserID lookup: a
		// failure here (e.g. the user has no open basket yet) is silently
		// ignored, same as the source's `if err == nil { ... }`.
		if basketID, err := h.BasketService.FindByUserID(r.Context(), user.ID); err == nil {
			cookie.Values[keys.SessionBasketID] = basketID
		}

		// Only the opaque session ID is added to the cookie's identity
		// slot — none of the tokens or PII in sess ever reach the
		// browser. Every other cookie.Values entry (basket/pagination/
		// search state, set above and elsewhere) is left untouched,
		// mirroring createSession in the source repo's cmd/web/helpers.go.
		cookie.Values[keys.SessionID] = sessionID
		if err := cookie.Save(r, w); err != nil {
			serverError(h.Logger, w, r, err)
			return
		}

		r = contextSetAuthenticatedUser(r, &user)

		if sess.UserDNI == "" {
			http.Redirect(w, r, "/user/update", http.StatusSeeOther)
			return
		}

		if sess.UserShippingAddress == "" {
			http.Redirect(w, r, "/user/create/address", http.StatusSeeOther)
			return
		}

		// Mirrors the source handler's late basket-quantity-cookie step:
		// by this point (fully onboarded), the cookie's basket ID must
		// already exist — a genuine error otherwise, unlike the early
		// lookup above.
		basketID, ok := getSessionString(cookie, keys.SessionBasketID)
		if !ok {
			serverError(h.Logger, w, r, errors.New("the basket creation process encountered an error"))
			return
		}

		basket, err := h.BasketService.FindByID(r.Context(), basketID)
		if err != nil {
			serverError(h.Logger, w, r, err)
			return
		}

		cookie.Values[keys.SessionBasketItems] = sumQuantities(basket.Items)
		if err := cookie.Save(r, w); err != nil {
			serverError(h.Logger, w, r, err)
			return
		}

		http.Redirect(w, r, "/search", http.StatusSeeOther)
	}
}

// ForgottenPassword mirrors the forgottenPassword handler in the source
// repo.
func (h *AuthHandler) ForgottenPassword(w http.ResponseWriter, r *http.Request) {
	var form struct {
		Email     string              `form:"Email"`
		Validator validator.Validator `form:"-"`
	}

	switch r.Method {
	case http.MethodGet:
		data := newTemplateData(r)
		data[web.Form] = form

		if err := response.Page(w, http.StatusOK, data, "pages/forgot-password-tmpl.html"); err != nil {
			serverError(h.Logger, w, r, err)
		}

	case http.MethodPost:
		if err := request.DecodePostForm(r, &form); err != nil {
			badRequest(h.Logger, w, r, err)
			return
		}

		form.Validator.CheckField(len(form.Email) != 0, web.Email, web.MsgEmailRequired)
		form.Validator.CheckField(validator.Matches(form.Email, validator.RgxEmail), web.Email, web.MsgInvalidEmail)

		if form.Validator.HasErrors() {
			data := newTemplateData(r)
			data[web.Form] = form

			if err := response.Page(w, http.StatusUnprocessableEntity, data, "pages/forgot-password-tmpl.html"); err != nil {
				serverError(h.Logger, w, r, err)
			}
			return
		}

		err := h.AuthService.ForgotPassword(r.Context(), form.Email)
		if err != nil {
			if errors.Is(err, domain.ErrUserNotFound) {
				form.Validator.CheckField(false, web.Email, web.MsgEmailNotMatching)

				data := newTemplateData(r)
				data[web.Form] = form

				if err := response.Page(w, http.StatusUnprocessableEntity, data, "pages/forgot-password-tmpl.html"); err != nil {
					serverError(h.Logger, w, r, err)
				}
				return
			}

			serverError(h.Logger, w, r, err)
			return
		}

		http.Redirect(w, r, "/password/forgot/confirmation", http.StatusSeeOther)
	}
}

// ForgottenPasswordConfirmation mirrors the forgottenPasswordConfirmation
// handler in the source repo.
func (h *AuthHandler) ForgottenPasswordConfirmation(w http.ResponseWriter, r *http.Request) {
	data := newTemplateData(r)

	if err := response.Page(w, http.StatusOK, data, "pages/reset-passwd-mail-confirm-tmpl.html"); err != nil {
		serverError(h.Logger, w, r, err)
	}
}

// PasswordReset mirrors the passwordReset handler in the source repo.
func (h *AuthHandler) PasswordReset(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "user_id")

	var form struct {
		Password        string              `form:"Password"`
		ConfirmPassword string              `form:"ConfirmPassword"`
		Validator       validator.Validator `form:"-"`
	}

	switch r.Method {
	case http.MethodGet:
		if len(userID) == 0 {
			serverError(h.Logger, w, r, errors.New("not valid user_id param"))
			return
		}

		data := newTemplateData(r)
		data[web.Form] = form
		data[web.UserID] = userID

		if err := response.Page(w, http.StatusOK, data, "pages/reset-password-tmpl.html"); err != nil {
			serverError(h.Logger, w, r, err)
			return
		}

	case http.MethodPost:
		if len(userID) == 0 {
			serverError(h.Logger, w, r, errors.New("not valid user_id param"))
			return
		}

		if err := request.DecodePostForm(r, &form); err != nil {
			badRequest(h.Logger, w, r, err)
			return
		}

		form.Validator.CheckField(len(form.Password) != 0, web.Password, web.MsgPasswdRequired)
		form.Validator.CheckField(len(form.Password) >= 8, web.Password, web.MsgTooShortPasswd)
		form.Validator.CheckField(len(form.Password) <= 72, web.Password, web.MsgTooLongPasswd)
		form.Validator.CheckField(validator.NotIn(form.Password, password.CommonPasswords...), web.Password, web.MsgTooCommonPasswd)

		form.Validator.CheckField(len(form.ConfirmPassword) != 0, web.ConfirmPassword, web.MsgPasswdRequired)
		form.Validator.CheckField(len(form.ConfirmPassword) >= 8, web.ConfirmPassword, web.MsgTooShortPasswd)
		form.Validator.CheckField(len(form.ConfirmPassword) <= 72, web.ConfirmPassword, web.MsgTooLongPasswd)
		form.Validator.CheckField(validator.NotIn(form.ConfirmPassword, password.CommonPasswords...), web.ConfirmPassword, web.MsgTooCommonPasswd)

		form.Validator.CheckField(validator.AreEqual(form.Password, form.ConfirmPassword), web.Password, web.MsgConfirmPasswdNotMatch)
		form.Validator.CheckField(validator.AreEqual(form.Password, form.ConfirmPassword), web.ConfirmPassword, web.MsgConfirmPasswdNotMatch)

		if form.Validator.HasErrors() {
			data := newTemplateData(r)
			data[web.Form] = form
			data[web.UserID] = userID

			if err := response.Page(w, http.StatusUnprocessableEntity, data, "pages/reset-password-tmpl.html"); err != nil {
				serverError(h.Logger, w, r, err)
			}
			return
		}

		if err := h.AuthService.ResetPassword(r.Context(), userID, form.Password, form.ConfirmPassword); err != nil {
			serverError(h.Logger, w, r, err)
			return
		}

		if err := response.Page(w, http.StatusOK, nil, "pages/new-passwd-confirm-tmpl.html"); err != nil {
			serverError(h.Logger, w, r, err)
		}
	}
}

// PasswordResetConfirmation mirrors the passwordResetConfirmation handler
// in the source repo.
func (h *AuthHandler) PasswordResetConfirmation(w http.ResponseWriter, r *http.Request) {
	data := newTemplateData(r)

	if err := response.Page(w, http.StatusOK, data, "pages/password-reset-confirmation-tmpl.html"); err != nil {
		serverError(h.Logger, w, r, err)
	}
}

// AccountActivateConfirmation mirrors the accountActivateConfirmation
// handler in the source repo.
func (h *AuthHandler) AccountActivateConfirmation(w http.ResponseWriter, r *http.Request) {
	data := newTemplateData(r)

	if err := response.Page(w, http.StatusOK, data, "pages/account-activated-confirmation-tmpl.html"); err != nil {
		serverError(h.Logger, w, r, err)
	}
}
