package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
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

// AddressHandler holds the chi handlers ported from the source repo's
// standalone address-management flow (cmd/web/address_handler.go's
// newAddress/updateAddress, reached via "/addresses/new" and
// "/addresses/{address_id}" in routes.go) plus the create-address wizard
// step of cmd/web/handlers.go (createAddressWizard/doneWizard, reached
// via "/user/create/address" and "/user/update/done").
//
// The wizard routes live under the "/user/..." URL prefix in the source
// repo's routes.go, but createAddressWizard's actual work — calling
// InsertAddress — is squarely an Address-slice concern (the same call
// newAddress makes), so it's grouped here with AddressService rather than
// in UserHandler, which only owns the DNI/phone/birthday profile update
// (UpdateUser). doneWizard has no downstream call of its own — it's kept
// alongside createAddressWizard since it's the wizard flow's next step,
// same grouping as the source file.
//
// Every handler here calls port/service.Address only; loading and
// re-saving the session record around CreateAddressWizard's call is still
// handler-level plumbing, mirroring the source's currentSession/
// updateSession helpers, same as UserHandler.
type AddressHandler struct {
	AddressService service.Address

	Sessions          *sessions.CookieStore
	SessionCookieName string
	SessionStore      resource.SessionStore
	SessionTTL        time.Duration

	Logger *slog.Logger
}

func NewAddressHandler(
	addressService service.Address,
	sessionStore resource.SessionStore,
	cookieStore *sessions.CookieStore,
	sessionTTL time.Duration,
	logger *slog.Logger,
) *AddressHandler {
	return &AddressHandler{
		AddressService:    addressService,
		Sessions:          cookieStore,
		SessionCookieName: keys.Session,
		SessionStore:      sessionStore,
		SessionTTL:        sessionTTL,
		Logger:            logger,
	}
}

// currentSession resolves the cookie's opaque session ID and loads the
// server-side domain.Session it points at — same reduced version (no
// access-token-refresh step) as UserHandler.currentSession; see its doc
// comment for why.
func (h *AddressHandler) currentSession(r *http.Request) (sessionID string, sess domain.Session, ok bool) {
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

// addressForm is shared by every handler in this file — the same field
// set the source repo's newAddress/updateAddress/createAddressWizard
// handlers each declare locally.
type addressForm struct {
	AddressID        string              `form:"AddressID"`
	Country          string              `form:"Country"`
	State            string              `form:"State"`
	City             string              `form:"City"`
	ZIP              string              `form:"ZIP"`
	MainAddress      string              `form:"MainAddress"`
	SecondaryAddress string              `form:"SecondaryAddress"`
	Numeration       string              `form:"Numeration"`
	Phone            string              `form:"Phone"`
	Validator        validator.Validator `form:"-"`
}

func checkAddressForm(form *addressForm) {
	form.Validator.CheckField(len(form.State) != 0, web.State, web.MsgStateRequired)
	form.Validator.CheckField(len(form.City) != 0, web.City, web.MsgCityRequired)
	form.Validator.CheckField(len(form.MainAddress) != 0, web.MainAddress, web.MsgMainAddressRequired)
	form.Validator.CheckField(len(form.Numeration) != 0, web.Numeration, web.MsgNumerationRequired)
	form.Validator.CheckField(len(form.SecondaryAddress) != 0, web.SecondaryAddress, web.MsgSecondaryAddressRequired)
	form.Validator.CheckField(len(form.Country) != 0 && !strings.Contains(form.Country, "Select Country"), web.Country, web.MsgCountryRequired)
	form.Validator.CheckField(len(form.Phone) != 0, web.RecipientPhoneNumber, web.MsgPhoneNumberRequired)
	form.Validator.CheckField(len(form.ZIP) != 0, web.ZIP, web.MsgZIPRequired)
}

// NewAddress mirrors the newAddress handler in the source repo's
// cmd/web/address_handler.go, reached via "/addresses/new".
func (h *AddressHandler) NewAddress(w http.ResponseWriter, r *http.Request) {
	_, sess, ok := h.currentSession(r)
	if !ok || sess.UserID == "" {
		serverError(h.Logger, w, r, keys.ErrSessionUserIDRequired)
		return
	}
	userID := sess.UserID

	var form addressForm

	// Mirrors the source handler exactly: the form is decoded regardless
	// of method, even for a bodyless GET.
	if err := request.DecodePostForm(r, &form); err != nil {
		badRequest(h.Logger, w, r, err)
		return
	}

	switch r.Method {
	case http.MethodGet:
		data := newTemplateData(r)
		data[web.Form] = form

		if err := response.Page(w, http.StatusOK, data, "pages/new-address-tmpl.html"); err != nil {
			serverError(h.Logger, w, r, err)
		}
		return

	case http.MethodPost:
		checkAddressForm(&form)

		if form.Validator.HasErrors() {
			data := newTemplateData(r)
			data[web.Form] = form

			if err := response.Page(w, http.StatusUnprocessableEntity, data, "pages/new-address-tmpl.html"); err != nil {
				serverError(h.Logger, w, r, err)
			}
			return
		}

		_, err := h.AddressService.Insert(r.Context(), userID, domain.ShippingAddress, form.Country, form.City, form.State, form.ZIP, form.MainAddress, form.SecondaryAddress, form.Numeration, form.Phone)
		switch {
		case err == nil:
			// Falls through to the redirect below, same as the source
			// handler's "case nil: break".

		case errors.Is(err, domain.ErrBadAddressType):
			badRequest(h.Logger, w, r, err)
			return

		case errors.Is(err, domain.ErrMaxAddressesReached):
			// Mirrors the source handler's special-case for this exact
			// downstream message: flag the MaxAddresses field instead of
			// rendering a generic error page. The source re-renders with
			// the downstream response's own status code
			// (errHTTP.StatusCode); that code isn't available behind the
			// domain sentinel here, so 422 is used instead — the same
			// status this handler already uses for every other
			// validation failure.
			form.Validator.CheckField(false, web.MaxAddresses, web.MsgAddressesLimit)

			data := newTemplateData(r)
			data[web.Form] = form

			if err := response.Page(w, http.StatusUnprocessableEntity, data, "pages/new-address-tmpl.html"); err != nil {
				serverError(h.Logger, w, r, err)
			}
			return

		default:
			serverError(h.Logger, w, r, err)
			return
		}
	}

	http.Redirect(w, r, "/checkout", http.StatusSeeOther)
}

// UpdateAddress mirrors the updateAddress handler in the source repo's
// cmd/web/address_handler.go, reached via "/addresses/{address_id}".
func (h *AddressHandler) UpdateAddress(w http.ResponseWriter, r *http.Request) {
	addressID := chi.URLParam(r, "address_id")
	if len(addressID) == 0 {
		serverError(h.Logger, w, r, errors.New("address_id is required"))
		return
	}

	var form addressForm

	_, sess, ok := h.currentSession(r)
	if !ok {
		serverError(h.Logger, w, r, keys.ErrSessionUserIDRequired)
		return
	}
	userID := sess.UserID

	user := contextGetAuthenticatedUser(r)
	if user == nil {
		serverError(h.Logger, w, r, errAuthenticatedUserRequired)
		return
	}

	addresses, err := h.AddressService.FindByUserID(r.Context(), userID)
	if err != nil {
		serverError(h.Logger, w, r, err)
		return
	}

	for _, a := range addresses {
		if a.ID == addressID {
			form.AddressID = addressID
			form.Country = a.Country
			form.State = a.State
			form.City = a.City
			form.ZIP = a.PostalCode
			form.MainAddress = a.MainStreet
			form.SecondaryAddress = a.SecondaryStreet
			form.Numeration = a.Numeration
			form.Phone = strings.TrimPrefix(a.Phone, "+593")
		}
	}

	if err := request.DecodePostForm(r, &form); err != nil {
		badRequest(h.Logger, w, r, err)
		return
	}

	switch r.Method {
	case http.MethodGet:
		data := newTemplateData(r)
		data[web.Form] = form

		if err := response.Page(w, http.StatusOK, data, "pages/address-tmpl.html"); err != nil {
			serverError(h.Logger, w, r, err)
		}
		return

	case http.MethodPost:
		checkAddressForm(&form)

		if form.Validator.HasErrors() {
			data := newTemplateData(r)
			data[web.Form] = form

			if err := response.Page(w, http.StatusUnprocessableEntity, data, "pages/address-tmpl.html"); err != nil {
				serverError(h.Logger, w, r, err)
			}
			return
		}

		if err := h.AddressService.Update(r.Context(), addressID, userID, domain.ShippingAddress, form.Country, form.City, form.State, form.ZIP, form.MainAddress, form.SecondaryAddress, form.Numeration, form.Phone); err != nil {
			serverError(h.Logger, w, r, err)
			return
		}

		http.Redirect(w, r, "/checkout", http.StatusSeeOther)
		return
	}
}

// CreateAddressWizard mirrors the createAddressWizard handler in the
// source repo's cmd/web/handlers.go, reached via "/user/create/address" —
// grouped here rather than in UserHandler despite its URL prefix; see
// this type's doc comment for why.
func (h *AddressHandler) CreateAddressWizard(w http.ResponseWriter, r *http.Request) {
	var form addressForm

	sessionID, sess, ok := h.currentSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	switch r.Method {
	case http.MethodGet:
		if sess.UserDNI == "" {
			http.Redirect(w, r, "/user/update", http.StatusSeeOther)
			return
		}

		if sess.UserShippingAddress != "" {
			http.Redirect(w, r, "/search", http.StatusSeeOther)
			return
		}

		data := newTemplateData(r)
		data[web.Form] = form

		if err := response.Page(w, http.StatusOK, data, "pages/address-wizard-tmpl.html"); err != nil {
			serverError(h.Logger, w, r, err)
		}
		return

	case http.MethodPost:
		if err := request.DecodePostForm(r, &form); err != nil {
			badRequest(h.Logger, w, r, err)
			return
		}

		checkAddressForm(&form)

		if form.Validator.HasErrors() {
			data := newTemplateData(r)
			data[web.Form] = form

			if err := response.Page(w, http.StatusUnprocessableEntity, data, "pages/address-wizard-tmpl.html"); err != nil {
				serverError(h.Logger, w, r, err)
			}
			return
		}

		addressID, err := h.AddressService.Insert(r.Context(), sess.UserID, domain.ShippingAddress, form.Country, form.City, form.State, form.ZIP, form.MainAddress, form.SecondaryAddress, form.Numeration, form.Phone)
		switch {
		case err == nil:
			if addressID != "" {
				sess.UserShippingAddress = addressID
				if err := h.SessionStore.Save(r.Context(), sessionID, sess, h.SessionTTL); err != nil {
					serverError(h.Logger, w, r, err)
					return
				}
			}

		case errors.Is(err, domain.ErrBadAddressType):
			badRequest(h.Logger, w, r, err)
			return

		default:
			serverError(h.Logger, w, r, err)
			return
		}
	}

	http.Redirect(w, r, "/user/update/done", http.StatusSeeOther)
}

// DoneWizard mirrors the doneWizard handler in the source repo's
// cmd/web/handlers.go, reached via "/user/update/done".
func (h *AddressHandler) DoneWizard(w http.ResponseWriter, r *http.Request) {
	data := newTemplateData(r)

	if err := response.Page(w, http.StatusOK, data, "pages/done-wizard-tmpl.html"); err != nil {
		serverError(h.Logger, w, r, err)
	}
}
