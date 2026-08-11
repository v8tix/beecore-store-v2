package response_test

import (
	"html/template"
	"testing"

	"github.com/v8tix/beecore-store-v2/assets"
	"github.com/v8tix/beecore-store-v2/internal/funcs"
)

// usedPageTemplates mirrors every literal "pages/*.html" string passed to
// response.Page/PageWithHeaders/PageRedirect across the handler package.
// If a handler starts referencing a template not in this list (or a listed
// one gets deleted), this test fails at build/test time instead of at the
// first live request — the exact failure mode the beecore-5 template
// cleanup ticket exists to guard against.
var usedPageTemplates = []string{
	"pages/account-activated-confirmation-tmpl.html",
	"pages/activate-tmpl.html",
	"pages/address-tmpl.html",
	"pages/address-wizard-tmpl.html",
	"pages/checkout-payphone-integrated-tmpl.html",
	"pages/done-wizard-tmpl.html",
	"pages/forgot-password-tmpl.html",
	"pages/landing-tmpl.html",
	"pages/login-tmpl-1.html",
	"pages/new-address-tmpl.html",
	"pages/new-passwd-confirm-tmpl.html",
	"pages/orders-list-tmpl.html",
	"pages/password-reset-confirmation-tmpl.html",
	"pages/payment-cancellation-tmpl.html",
	"pages/payment-confirmation-tmpl.html",
	"pages/product-details-tmpl.html",
	"pages/product-grid-tmpl.html",
	"pages/reset-passwd-mail-confirm-tmpl.html",
	"pages/reset-password-tmpl.html",
	"pages/shopping-cart-tmpl.html",
	"pages/signup-tmpl-1.html",
	"pages/user-wizard-tmpl.html",
}

func TestUsedPageTemplates_ParseSuccessfully(t *testing.T) {
	for _, page := range usedPageTemplates {
		t.Run(page, func(t *testing.T) {
			patterns := []string{
				"templates/base-tmpl-1.html",
				"templates/partials/*.html",
				"templates/" + page,
			}

			_, err := template.New("").Funcs(funcs.TemplateFuncs).ParseFS(assets.EmbeddedFiles, patterns...)
			if err != nil {
				t.Fatalf("failed to parse %s: %v", page, err)
			}
		})
	}
}
