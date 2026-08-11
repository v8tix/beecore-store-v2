package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/sessions"

	httputils "github.com/v8tix/beecore-http/utils"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/core/port/service"
	"github.com/v8tix/beecore-store-v2/internal/foundation/keys"
	"github.com/v8tix/beecore-store-v2/internal/foundation/web"
	"github.com/v8tix/beecore-store-v2/internal/request"
	"github.com/v8tix/beecore-store-v2/internal/response"
	"github.com/v8tix/beecore-store-v2/internal/validator"
)

// productPageSize mirrors the pageSize const in the source repo's
// cmd/web/products_handler.go — the number of results the downstream
// beecore-stores search endpoint returns per page, used only to compute
// TotalPages/HasNextPage/HasPreviousPage client-side (the actual page-size
// enforcement happens downstream, not here).
const productPageSize = 10

// ProductHandler holds the chi handlers ported from cmd/web/products_handler.go
// in the source repo's findProducts ("/search") and productDetails
// ("/products/{product_id}") — the storefront's product search/pagination
// and product-detail pages. It calls port/service.Product only.
//
// searchAddItem ("/search/add" in the source repo's routes.go) is NOT
// ported here: despite living in the same source file, its actual work —
// BasketAddItem/FindBasket/CreateBasket — is squarely a Basket-slice
// concern, not a Product one, and resource/service.Basket don't exist yet
// (plan Task 13/14). It's picked up once the Basket slice lands, the same
// deferral AuthHandler.Login and UserHandler.UpdateWizard already document
// for their own basket-dependent steps.
//
// Unlike AddressHandler/UserHandler, ProductHandler never needs the
// Redis-backed resource.SessionStore: neither findProducts nor
// productDetails reads or writes domain.Session (no identity lookup) —
// both only touch the browser cookie's own session.Values directly
// (BasketItems/UserInput/SelfPage/NextPage/PreviousPage), the same
// non-identity keys keys.go's doc comment describes as "deliberately left
// in the cookie", mirroring AuthHandler's Sessions-only shape.
type ProductHandler struct {
	ProductService service.Product

	Sessions          *sessions.CookieStore
	SessionCookieName string

	Logger *slog.Logger
}

func NewProductHandler(
	productService service.Product,
	sessionStore *sessions.CookieStore,
	logger *slog.Logger,
) *ProductHandler {
	return &ProductHandler{
		ProductService:    productService,
		Sessions:          sessionStore,
		SessionCookieName: keys.Session,
		Logger:            logger,
	}
}

// productSearchForm is findProducts' form, ported field-for-field from its
// anonymous struct in the source repo — Products holds domain.Product
// instead of stores.GetProductV1Res.
type productSearchForm struct {
	Search          string              `form:"Search"`
	Products        []domain.Product    `form:"Products"`
	NextPage        int64               `form:"NextPage"`
	HasNextPage     bool                `form:"HasNextPage"`
	PreviousPage    int64               `form:"PreviousPage"`
	HasPreviousPage bool                `form:"HasPreviousPage"`
	SelfPage        int64               `form:"SelfPage"`
	TotalPages      int64               `form:"TotalPages"`
	IsMobile        bool                `form:"IsMobile"`
	BasketItems     int                 `form:"BasketItems"`
	Validator       validator.Validator `form:"-"`
}

// renderProductSearch mirrors the fetch-paginate-render sequence the
// source repo's findProducts handler repeats identically in three
// branches (GET with a "page" query param, GET falling back to the
// session's previously-stored page, and POST with a non-empty search
// term): call ProductService.Search for form.SelfPage-1 (zero-based),
// compute pagination state via beecore-http/utils exactly as the source
// does, persist it into the browser session cookie, and render the
// product grid template. Consolidated into one method instead of
// triplicated inline, but the sequence of operations and their outcomes
// are unchanged from the source.
func (h *ProductHandler) renderProductSearch(w http.ResponseWriter, r *http.Request, session *sessions.Session, form *productSearchForm) {
	page, err := h.ProductService.Search(r.Context(), form.Search, form.SelfPage-1)
	if err != nil {
		serverError(h.Logger, w, r, err)
		return
	}

	form.TotalPages = httputils.NumberOfPages(page.Total, int64(productPageSize))

	if previousPage, ok := httputils.HasPreviousPage(form.SelfPage); ok {
		session.Values[keys.SessionPreviousPage] = previousPage
		form.PreviousPage = previousPage
		form.HasPreviousPage = true
	} else {
		form.PreviousPage = 1
		form.HasPreviousPage = false
	}

	if nextPage, ok := httputils.HasNextPage(form.TotalPages, form.SelfPage); ok {
		session.Values[keys.SessionNextPage] = nextPage
		form.NextPage = nextPage
		form.HasNextPage = true
	} else {
		form.NextPage = form.TotalPages
		form.HasNextPage = false
	}

	if err := session.Save(r, w); err != nil {
		serverError(h.Logger, w, r, err)
		return
	}

	form.Products = page.Products
	data := newTemplateData(r)
	data[web.Form] = form

	if err := response.Page(w, http.StatusOK, data, "pages/product-grid-tmpl.html"); err != nil {
		serverError(h.Logger, w, r, err)
	}
}

// FindProducts mirrors the findProducts handler in the source repo,
// reached via GET/POST "/search". Preserves the source's control flow
// 1:1, including its quirks: the GET branch returns without ever writing
// a response when the session has a "page" (or previously-stored
// SelfPage) but no stored search term yet (session.Values[SessionUserInput]
// not set) — that's an existing source behavior, not something introduced
// here — and the POST branch's form.Validator.HasErrors() check is dead
// code in both versions (findProducts never calls
// form.Validator.CheckField, so HasErrors() is always false), preserved
// rather than removed since 1:1 porting keeps unreachable branches intact
// as documentation of the source's shape.
func (h *ProductHandler) FindProducts(w http.ResponseWriter, r *http.Request) {
	var form productSearchForm

	if strings.Contains(r.Header.Get("User-Agent"), "Mobile") {
		form.IsMobile = true
	}

	session, err := h.Sessions.Get(r, h.SessionCookieName)
	if err != nil {
		serverError(h.Logger, w, r, err)
		return
	}

	if items, ok := session.Values[keys.SessionBasketItems]; ok {
		if n, ok := items.(int); ok {
			form.BasketItems = n
		}
	}

	if err := request.DecodePostForm(r, &form); err != nil {
		badRequest(h.Logger, w, r, err)
		return
	}

	switch r.Method {
	case http.MethodGet:
		page := httputils.GetQueryParam(r, "page")
		if len(page) > 0 {
			parsed, err := strconv.ParseInt(page, 10, 64)
			if err != nil {
				serverError(h.Logger, w, r, err)
				return
			}

			session.Values[keys.SessionSelfPage] = parsed
			form.SelfPage = parsed

			if userInput, ok := session.Values[keys.SessionUserInput].(string); ok {
				form.Search = userInput
				h.renderProductSearch(w, r, session, &form)
			}
			return
		}

		if selfPage, ok := session.Values[keys.SessionSelfPage].(int64); ok {
			form.SelfPage = selfPage

			if userInput, ok := session.Values[keys.SessionUserInput].(string); ok {
				form.Search = userInput
				h.renderProductSearch(w, r, session, &form)
			}
			return
		}

		data := newTemplateData(r)
		data[web.Form] = form

		if err := response.Page(w, http.StatusOK, data, "pages/product-grid-tmpl.html"); err != nil {
			serverError(h.Logger, w, r, err)
		}

	case http.MethodPost:
		if form.Validator.HasErrors() {
			data := newTemplateData(r)
			data[web.Form] = form

			if err := response.Page(w, http.StatusUnprocessableEntity, data, "pages/product-grid-tmpl.html"); err != nil {
				serverError(h.Logger, w, r, err)
				return
			}
			// No return here — mirrors the source handler exactly, which
			// falls through to the code below even after rendering the
			// 422. Unreachable in practice (see this method's doc
			// comment), preserved rather than fixed.
		}

		if len(form.Search) != 0 {
			session.Values[keys.SessionUserInput] = form.Search
			session.Values[keys.SessionSelfPage] = int64(1)
			form.SelfPage = 1

			h.renderProductSearch(w, r, session, &form)
			return
		}

		data := newTemplateData(r)
		data[web.Form] = form

		if err := response.Page(w, http.StatusOK, data, "pages/product-grid-tmpl.html"); err != nil {
			serverError(h.Logger, w, r, err)
		}
	}
}

// ProductDetails mirrors the productDetails handler in the source repo,
// reached via GET "/products/{product_id}".
func (h *ProductHandler) ProductDetails(w http.ResponseWriter, r *http.Request) {
	session, err := h.Sessions.Get(r, h.SessionCookieName)
	if err != nil {
		serverError(h.Logger, w, r, err)
		return
	}

	productID := chi.URLParam(r, "product_id")
	if len(productID) == 0 {
		serverError(h.Logger, w, r, errors.New("product_id is required"))
		return
	}

	var form struct {
		Product     domain.Product      `form:"Product"`
		BasketItems int                 `form:"BasketItems"`
		IsMobile    bool                `form:"IsMobile"`
		Validator   validator.Validator `form:"-"`
	}

	if items, ok := session.Values[keys.SessionBasketItems]; ok {
		if n, ok := items.(int); ok {
			form.BasketItems = n
		}
	}

	if strings.Contains(r.Header.Get("User-Agent"), "Mobile") {
		form.IsMobile = true
	}

	product, err := h.ProductService.FindByID(r.Context(), productID)
	if err != nil {
		serverError(h.Logger, w, r, err)
		return
	}
	form.Product = product

	if err := request.DecodePostForm(r, &form); err != nil {
		badRequest(h.Logger, w, r, err)
		return
	}

	if r.Method == http.MethodGet {
		data := newTemplateData(r)
		data[web.Form] = form

		if err := response.Page(w, http.StatusOK, data, "pages/product-details-tmpl.html"); err != nil {
			serverError(h.Logger, w, r, err)
		}
	}
}
