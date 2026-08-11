package web

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/v8tix/beecore-store-v2/assets"
)

// routes mirrors application.routes in the source repo's cmd/web/routes.go
// 1:1 — every route below points at the equivalent vertical-slice handler
// (package handler) instead of the source's single application struct's
// methods; home/logout, which have no vertical-slice owner, point at this
// package's own app.home/app.logout (home.go). PayPal routes (there were
// none dedicated to it in the source — checkout_handler.go's dispatch was
// the only PayPal-aware code in routing) are correctly absent; every
// "/payphone/*" and "/checkout" route below points at PaymentHandler.
func (a *app) routes() http.Handler {
	mux := chi.NewRouter()
	mux.NotFound(a.notFound)

	// Global middleware in careful order
	mux.Use(a.recoverPanic) // Keep panic recovery early in the chain
	mux.Use(a.securityHeaders)
	mux.Use(a.metrics) // Add metrics tracking
	// Static file handling with caching
	fileServer := http.FileServer(http.FS(assets.EmbeddedFiles))
	mux.Handle("/static/*", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//* Images, fonts, JS, CSS: 1 week cache with validation (public, max-age=604800, must-revalidate)
		//* Other static resources: 1 day cache with validation (public, max-age=86400, must-revalidate)
		//* must-revalidate directive to all cache control headers is a good practice because:
		//	1. It forces browsers to check with the server when the cache expires
		//	2. It prevents browsers from using stale content if network connectivity is limited
		//	3. It ensures your users always get the most up-to-date resources after the cache period

		// Apply cache headers for static assets
		if strings.HasPrefix(r.URL.Path, "/static/") {
			// Check if this is an image, font, or other highly cacheable content
			if strings.HasSuffix(r.URL.Path, ".jpg") ||
				strings.HasSuffix(r.URL.Path, ".png") ||
				strings.HasSuffix(r.URL.Path, ".gif") ||
				strings.HasSuffix(r.URL.Path, ".ico") ||
				strings.HasSuffix(r.URL.Path, ".woff") ||
				strings.HasSuffix(r.URL.Path, ".woff2") {
				// Images and fonts - 1 week with validation
				w.Header().Set("Cache-Control", "public, max-age=604800, must-revalidate") // 1 week
			} else if strings.HasSuffix(r.URL.Path, ".js") ||
				strings.HasSuffix(r.URL.Path, ".css") {
				// Moderate cache for JS and CSS - but include validation
				w.Header().Set("Cache-Control", "public, max-age=604800, must-revalidate") // 1 week
			} else {
				// Default shorter cache for other resources
				w.Header().Set("Cache-Control", "public, max-age=86400, must-revalidate") // 1 day
			}
			// Add Vary header to respect different accept-encoding values
			w.Header().Add("Vary", "Accept-Encoding")
		}

		fileServer.ServeHTTP(w, r)
	}))

	mux.Group(func(mux chi.Router) {
		mux.Use(a.preventCSRF)
		mux.Use(a.authenticate)

		mux.Group(func(mux chi.Router) {
			mux.Use(a.requireAnonymousUser)
			mux.Get("/", a.home)
			mux.Get("/login", a.authHandler.Login)
			mux.Post("/login", a.authHandler.Login)
			mux.Get("/signup", a.authHandler.Signup)
			mux.Post("/signup", a.authHandler.Signup)
			mux.Get("/activate/{user_id}", a.authHandler.Activate)
			mux.Post("/activate/{user_id}", a.authHandler.Activate)
			mux.Get("/account/activated", a.authHandler.AccountActivateConfirmation)
			mux.Get("/password/forgot", a.authHandler.ForgottenPassword)
			mux.Post("/password/forgot", a.authHandler.ForgottenPassword)
			mux.Get("/password/forgot/confirmation", a.authHandler.ForgottenPasswordConfirmation)
			mux.Get("/password/reset/{user_id}", a.authHandler.PasswordReset)
			mux.Post("/password/reset/{user_id}", a.authHandler.PasswordReset)
			mux.Get("/password/reset/confirmation", a.authHandler.PasswordResetConfirmation)
		})

		mux.Group(func(mux chi.Router) {
			mux.Use(a.requireAuthenticatedUser)
			mux.Get("/search", a.productHandler.FindProducts)
			mux.Post("/search", a.productHandler.FindProducts)
			mux.Get("/logout", a.logout)
			mux.Get("/user/update", a.userHandler.UpdateWizard)
			mux.Post("/user/update", a.userHandler.UpdateWizard)
			mux.Get("/user/create/address", a.addressHandler.CreateAddressWizard)
			mux.Post("/user/create/address", a.addressHandler.CreateAddressWizard)
			mux.Get("/user/update/done", a.addressHandler.DoneWizard)
			mux.Get("/basket", a.basketHandler.Basket)
			mux.Post("/basket", a.basketHandler.Basket)
			mux.Post("/basket/add-item/{product_id}", a.basketHandler.AddItem)
			mux.Post("/basket/remove-item/{product_id}", a.basketHandler.RemoveItem)
			mux.Get("/search/add", a.basketHandler.SearchAddItem)
			mux.Get("/checkout", a.paymentHandler.Checkout)
			mux.Post("/checkout", a.paymentHandler.Checkout)
			mux.Post("/payphone/create-payment", a.paymentHandler.ProcessPayment)
			mux.Get("/payphone/confirm", a.paymentHandler.ConfirmPayment)
			mux.Get("/payphone/cancel", a.paymentHandler.CancelPayment)
			mux.Get("/orders/confirm", a.orderHandler.ConfirmOrder)
			mux.Get("/orders/cancel", a.orderHandler.CancelOrder)
			mux.Get("/addresses/new", a.addressHandler.NewAddress)
			mux.Post("/addresses/new", a.addressHandler.NewAddress)
			mux.Get("/addresses/{address_id}", a.addressHandler.UpdateAddress)
			mux.Post("/addresses/{address_id}", a.addressHandler.UpdateAddress)
			mux.Get("/products/{product_id}", a.productHandler.ProductDetails)
			mux.Get("/orders", a.orderHandler.Orders)
		})
	})

	return mux
}
