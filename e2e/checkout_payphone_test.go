//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestCheckoutPayPhone_CompletesPurchaseAndConfirmsOrder drives a full
// buyer checkout against a live beecore-store-v2 (and whatever
// beecore-baskets/beecore-ordering/beecore-payments it's pointed at), the
// same way TestTaxonomyComboBox_FiltersAndSelectsLeaf
// (beecore-admin-v2/e2e) drives the add-product page's category combo
// box: log in, navigate the real flow, read real DOM/network state back,
// assert on it.
//
// It exists as regression coverage for two bugs found and fixed while
// implementing the payment-record wiring this test exercises
// (core/payment.Service's and resource.PaymentRecordRemote's doc
// comments have the full story):
//
//  1. ProcessPayment's handler never resolved the logged-in buyer's
//     UserID, so the downstream beecore-payments call it now makes failed
//     validation ("user_id: must be provided") on every real checkout —
//     visibly, as a "Payment service is temporarily unavailable" banner
//     the instant a shipping address was selected. This test's first
//     assertion (PayPhone's widget becomes ready, not that banner) directly
//     pins that regression.
//  2. Before either fix, beecore-payments was never called at all —
//     invisible from the UI, since the checkout flow still completed and
//     showed a success page regardless (a bogus client-side payment
//     reference flowed into the Basket/Order either way). A pure
//     UI-driven test can't tell that apart from a real fix, which is why
//     this is in beecore-store-v2/e2e rather than claimed as sufficient
//     proof on its own — see beecore-store-v2's internal/core/payment
//     package tests and internal/infrastructure/http/paymentrecordremote's
//     tests for the coverage that actually pins the payment record itself
//     getting created/confirmed.
//
// Requires a buyer account (E2E_EMAIL/E2E_PASSWORD) that owns at least one
// saved shipping address and can find at least one in-stock product via
// E2E_PRODUCT_QUERY (default "a" — broad enough to match most catalogs;
// override it for an empty/unusual one).
func TestCheckoutPayPhone_CompletesPurchaseAndConfirmsOrder(t *testing.T) {
	baseURL, email, password := config(t)
	s, ctx := newSession(t)

	productQuery := os.Getenv("E2E_PRODUCT_QUERY")
	if productQuery == "" {
		productQuery = "a"
	}

	pageID, err := openPage(ctx, s, baseURL+"/login")
	if err != nil {
		t.Fatalf("opening the login page: %v", err)
	}

	loginJS := fmt.Sprintf(`() => {
		document.querySelector('input[name="Email"]').value = %s;
		document.querySelector('input[name="Password"]').value = %s;
		document.querySelector('button[name="submit"]').click();
	}`, strconv.Quote(email), strconv.Quote(password))

	if err := eval(ctx, s, pageID, loginJS, true, nil); err != nil {
		t.Fatalf("submitting the login form: %v", err)
	}

	var path string
	if err := eval(ctx, s, pageID, `() => location.pathname`, false, &path); err != nil {
		t.Fatalf("reading the post-login URL: %v", err)
	}
	if path == "/login" {
		t.Fatalf("still on /login after submitting -- login failed (check E2E_EMAIL/E2E_PASSWORD)")
	}

	// The search box is a real <form method="POST" action="/search"> (see
	// its name="Search" field) -- a GET with a query string never reaches
	// the handler's decoded form, so this submits it the way a real
	// shopper would, same as loginJS above.
	if _, err := s.call(ctx, "navigate_page", map[string]any{
		"pageId": pageID, "type": "url", "url": baseURL + "/search",
	}); err != nil {
		t.Fatalf("navigating to /search: %v", err)
	}

	searchJS := fmt.Sprintf(`() => {
		document.querySelector('input[name="Search"]').value = %s;
		document.querySelector('form[action="/search"] button[type="submit"]').click();
	}`, strconv.Quote(productQuery))

	if err := eval(ctx, s, pageID, searchJS, true, nil); err != nil {
		t.Fatalf("submitting the search for %q: %v", productQuery, err)
	}

	// Add the first result to the cart via its real "Add to Cart" link
	// (/search/add?item=<id>, a plain GET) rather than clicking it -- the
	// product actually found depends on whatever the live catalog has,
	// so there's no fixed uid/selector to click, and a direct navigation
	// is less flaky than locating and clicking a specific card in a
	// results grid.
	var addToCartHref string
	if err := eval(ctx, s, pageID, `() => {
		const a = document.querySelector('a[href*="/search/add"]');
		return a ? a.getAttribute('href') : "";
	}`, false, &addToCartHref); err != nil {
		t.Fatalf("reading the first result's Add to Cart link: %v", err)
	}
	if addToCartHref == "" {
		t.Fatalf("no product matched query %q -- set E2E_PRODUCT_QUERY to something the live catalog has in stock", productQuery)
	}

	if _, err := s.call(ctx, "navigate_page", map[string]any{
		"pageId": pageID, "type": "url", "url": baseURL + addToCartHref,
	}); err != nil {
		t.Fatalf("adding the product to the cart: %v", err)
	}

	if _, err := s.call(ctx, "navigate_page", map[string]any{
		"pageId": pageID, "type": "url", "url": baseURL + "/checkout",
	}); err != nil {
		t.Fatalf("navigating to /checkout: %v", err)
	}

	snap, err := snapshot(ctx, s, pageID)
	if err != nil {
		t.Fatalf("taking a snapshot of /checkout: %v", err)
	}
	addressUID, ok := findUID(snap, "radio")
	if !ok {
		t.Fatalf("no shipping address radio button found -- does %s own at least one saved address?", email)
	}

	if err := click(ctx, s, pageID, addressUID); err != nil {
		t.Fatalf("selecting the shipping address: %v", err)
	}

	// Regression pin for bug 1 (see this test's doc comment): before the
	// handler fix, selecting an address always surfaced this banner
	// instead of ever reaching a ready PayPhone widget, because
	// ProcessPayment's downstream call failed validation on every real
	// attempt.
	if err := waitUntil(ctx, 10*time.Second, func() (bool, error) {
		var bodyText string
		if err := eval(ctx, s, pageID, `() => document.body.innerText`, false, &bodyText); err != nil {
			return false, err
		}
		if strings.Contains(bodyText, "Payment service is temporarily unavailable") {
			return false, fmt.Errorf("checkout surfaced the payment-service-unavailable banner -- " +
				"ProcessPayment's downstream beecore-payments call is failing again")
		}
		return strings.Contains(bodyText, "PayPhone ready"), nil
	}); err != nil {
		t.Fatalf("waiting for the PayPhone widget to become ready: %v", err)
	}

	// The widget's card fields just mounted -- previous uids (including
	// addressUID above) may now point at replaced nodes, so snapshot
	// again before looking for them.
	snap, err = snapshot(ctx, s, pageID)
	if err != nil {
		t.Fatalf("taking a snapshot after the PayPhone widget mounted: %v", err)
	}

	cardNumberUID, ok := findUID(snap, "número de tarjeta")
	if !ok {
		t.Fatal("card number field not found in the PayPhone widget")
	}
	expiryUID, ok := findUID(snap, "mm/aa")
	if !ok {
		t.Fatal("expiration date field not found in the PayPhone widget")
	}
	cvvUID, ok := findUID(snap, "cvv")
	if !ok {
		t.Fatal("CVV field not found in the PayPhone widget")
	}
	cardholderUID, ok := findUID(snap, "titular de tarjeta")
	if !ok {
		t.Fatal("cardholder name field not found in the PayPhone widget")
	}
	payUID, ok := findUID(snap, "pagar")
	if !ok {
		t.Fatal("\"Pagar\" button not found in the PayPhone widget")
	}

	// PayPhone's own published test card — see
	// https://docs.payphone.app (sandbox test cards): any future
	// expiration, any 3-digit CVV.
	for _, f := range []struct{ uid, value string }{
		{cardNumberUID, "4111111111111111"},
		{expiryUID, "12/30"},
		{cvvUID, "123"},
		{cardholderUID, "Marco Almeida"},
	} {
		if err := fill(ctx, s, pageID, f.uid, f.value); err != nil {
			t.Fatalf("filling a PayPhone card field (uid %s): %v", f.uid, err)
		}
	}

	if err := click(ctx, s, pageID, payUID); err != nil {
		t.Fatalf("clicking Pagar: %v", err)
	}

	// PayPhone's confirmation round trip (ConfirmPayment: find the
	// payment record, confirm with PayPhone, update it, create the
	// order(s), check out the basket, open a fresh one) crosses a real
	// page navigation (/payphone/confirm redirecting to /orders/confirm),
	// which wait_for's text match doesn't reliably track across in this
	// pinned chrome-devtools-mcp release -- polling location.pathname via
	// eval instead, the same waitUntil+eval pattern
	// TestTaxonomyComboBox_FiltersAndSelectsLeaf uses for its own
	// hidden-vs-shown check above.
	err = waitUntil(ctx, 20*time.Second, func() (bool, error) {
		if err := eval(ctx, s, pageID, `() => location.pathname`, false, &path); err != nil {
			return false, err
		}
		return path == "/orders/confirm", nil
	})
	if err != nil {
		t.Fatalf("waiting for the order confirmation page (last path seen: %q): %v", path, err)
	}

	var confirmationText string
	if err := eval(ctx, s, pageID, `() => document.body.innerText`, false, &confirmationText); err != nil {
		t.Fatalf("reading the confirmation page's text: %v", err)
	}
	if !strings.Contains(confirmationText, "Your order has been placed successfully!") {
		t.Fatalf("landed on /orders/confirm but its text doesn't contain the expected success message: %q", confirmationText)
	}
}
