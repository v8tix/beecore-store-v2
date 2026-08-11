// Internal (white-box) test package, not payphoneremote_test: unlike this
// repo's other resource adapters, PayPhone's confirmation endpoint is a
// hardcoded literal (payPhoneConfirmAPIURL, mirroring the source repo's
// PayPhoneRepositoryStrategy.callConfirmAPI — see that var's doc comment
// in payphoneremote.go for why it's a var, not a const), not a
// config-injected URL like Integration.V1.OrdersURL. Being in-package lets
// TestConfirmPayment_Success/Error swap it for an httptest.Server for the
// duration of the test and restore it after, without changing what
// production code points at by default. It also lets the DTO-mapping unit
// tests (TestTransformOrder/TestGenerateReference/TestValidatePayPhoneOrder)
// exercise the unexported inlined-transform helpers directly — the whole
// point of Task 17 is preserving that field-for-field mapping logic
// exactly, so it gets tested at that level, not just indirectly through
// ProcessPayment.
package payphoneremote

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/v8tix/beecore-eda/config"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
)

func newTestClient(cfg *config.Cfg) *Client {
	if cfg.Web.HTTPClient == nil {
		cfg.Web.HTTPClient = http.DefaultClient
	}
	return NewClient(cfg)
}

func financialsWithOneProduct() domain.Financials {
	return domain.Financials{
		Items: []domain.FinancialLineItem{
			{
				Store: domain.Store{ID: "s1", Name: "Acme"},
				Products: []domain.BasketProduct{
					{ID: "p1", Name: "Widget", Description: "A widget", Price: 10, Subtotal: 20, Quantity: 2},
				},
				ShippingAmount: 1,
				TaxesAmount:    2,
				Subtotal:       20,
				Total:          23,
			},
		},
		Subtotal:            20,
		TotalShippingAmount: 1,
		TotalTaxesAmount:    2,
		Total:               23,
	}
}

func TestProcessPayment_Success(t *testing.T) {
	c := newTestClient(&config.Cfg{
		Payphone: config.Payphone{BaseURL: "https://store.example.com"},
		BusinessParameters: config.BusinessParameters{
			Country: config.Country{Taxes: config.Taxes{Percentage: 12}},
		},
	})

	req := domain.PaymentRequest{
		OrderID:             "order-uuid-1",
		ClientTransactionID: "ctx-1",
		Currency:            "USD",
		Financials:          financialsWithOneProduct(),
	}

	got, err := c.ProcessPayment(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.PaymentID != "ctx-1" {
		t.Errorf("PaymentID = %q, want %q", got.PaymentID, "ctx-1")
	}
	if got.OrderID != "order-uuid-1" {
		t.Errorf("OrderID = %q, want %q", got.OrderID, "order-uuid-1")
	}
	if got.Status != "CREATED" {
		t.Errorf("Status = %q, want %q", got.Status, "CREATED")
	}
	if got.RedirectURL != "" {
		t.Errorf("RedirectURL = %q, want empty (PayPhone doesn't redirect)", got.RedirectURL)
	}
	if got.ConfirmURL != "https://store.example.com/payphone/confirm" {
		t.Errorf("ConfirmURL = %q", got.ConfirmURL)
	}
	if got.CancelURL != "https://store.example.com/payphone/cancel" {
		t.Errorf("CancelURL = %q", got.CancelURL)
	}

	order, ok := got.Metadata["payphoneOrder"].(payPhoneOrderRequest)
	if !ok {
		t.Fatalf("Metadata[payphoneOrder] missing or wrong type: %+v", got.Metadata)
	}
	if order.ClientTransactionID != "order-uuid-1" {
		t.Errorf("order.ClientTransactionID = %q, want req.OrderID %q", order.ClientTransactionID, "order-uuid-1")
	}
	if order.Amount != 20 || order.Tax != 2 || order.Service != 1 {
		t.Errorf("unexpected order amounts: %+v", order)
	}
	if len(order.Products) != 1 {
		t.Fatalf("expected 1 product, got %d", len(order.Products))
	}
	// Price 10, taxRate 0.12 -> tax = 1.2
	if order.Products[0].Tax != 1.2 {
		t.Errorf("Products[0].Tax = %v, want 1.2", order.Products[0].Tax)
	}
}

func TestProcessPayment_NoProducts(t *testing.T) {
	c := newTestClient(&config.Cfg{})

	_, err := c.ProcessPayment(t.Context(), domain.PaymentRequest{
		ClientTransactionID: "ctx-1",
		Financials:          domain.Financials{Total: 10},
	})
	if !errors.Is(err, ErrProductsRequired) {
		t.Fatalf("got %v, want ErrProductsRequired", err)
	}
}

func TestProcessPayment_InvalidTotal(t *testing.T) {
	c := newTestClient(&config.Cfg{})

	_, err := c.ProcessPayment(t.Context(), domain.PaymentRequest{
		ClientTransactionID: "ctx-1",
		Financials:          domain.Financials{Items: []domain.FinancialLineItem{{Products: []domain.BasketProduct{{Name: "x", Quantity: 1}}}}, Total: 0},
	})
	if !errors.Is(err, ErrOrderTotalInvalid) {
		t.Fatalf("got %v, want ErrOrderTotalInvalid", err)
	}
}

func TestProcessPayment_MissingClientTransactionID(t *testing.T) {
	c := newTestClient(&config.Cfg{})

	_, err := c.ProcessPayment(t.Context(), domain.PaymentRequest{
		Financials: domain.Financials{
			Items: []domain.FinancialLineItem{{Products: []domain.BasketProduct{{Name: "x", Price: 1, Quantity: 1}}}},
			Total: 10,
		},
	})
	if !errors.Is(err, ErrMissingClientTxID) {
		t.Fatalf("got %v, want ErrMissingClientTxID", err)
	}
}

// TestConfirmPayment_Success proves ConfirmPayment maps a PayPhone
// confirmation response onto domain.PaymentConfirmation field-for-field,
// including the amount-from-cents conversion, exactly like the source's
// PayPhoneRepositoryStrategy.ConfirmPayment.
func TestConfirmPayment_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/api/button/V2/Confirm" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer ph-token" {
			t.Errorf("unexpected auth header: %s", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("unexpected content-type header: %s", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"email":"buyer@example.com",
			"cardType":"Credit",
			"lastDigits":"1234",
			"cardBrand":"Visa",
			"amount":2300,
			"clientTransactionId":"ctx-1",
			"phoneNumber":"0999999999",
			"statusCode":3,
			"transactionStatus":"Approved",
			"authorizationCode":"AUTH1",
			"transactionId":987,
			"document":"1234567890",
			"currency":"USD",
			"date":"2026-01-01",
			"reference":"Order ref"
		}`))
	}))
	defer ts.Close()

	restore := setConfirmAPIURLForTest(t, ts.URL+"/api/button/V2/Confirm")
	defer restore()

	c := newTestClient(&config.Cfg{Payphone: config.Payphone{Token: "ph-token"}})

	got, err := c.ConfirmPayment(t.Context(), "987", "ctx-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := domain.PaymentConfirmation{
		TransactionID:     "987",
		PaymentID:         "ctx-1",
		Status:            "Approved",
		Amount:            23,
		Currency:          "USD",
		AuthorizationCode: "AUTH1",
		CardType:          "Credit",
		CardBrand:         "Visa",
		LastDigits:        "1234",
		Email:             "buyer@example.com",
		PhoneNumber:       "0999999999",
		Document:          "1234567890",
		Reference:         "Order ref",
		Date:              "2026-01-01",
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

// TestConfirmPayment_NotApproved proves a non-approved response is
// surfaced via ErrorMessage rather than as a Go error, mirroring the
// source's ConfirmPayment: the caller (handler/use-case) decides what to
// do with an unsuccessful-but-well-formed confirmation.
func TestConfirmPayment_NotApproved(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"statusCode":5,"transactionStatus":"Rejected","transactionId":1,"clientTransactionId":"ctx-1"}`))
	}))
	defer ts.Close()

	restore := setConfirmAPIURLForTest(t, ts.URL)
	defer restore()

	c := newTestClient(&config.Cfg{Payphone: config.Payphone{Token: "ph-token"}})

	got, err := c.ConfirmPayment(t.Context(), "1", "ctx-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ErrorMessage != "Payment was not approved" {
		t.Fatalf("ErrorMessage = %q", got.ErrorMessage)
	}
}

func TestConfirmPayment_MissingParams(t *testing.T) {
	c := newTestClient(&config.Cfg{})

	if _, err := c.ConfirmPayment(t.Context(), "", "ctx-1"); !errors.Is(err, ErrMissingPayPhoneParams) {
		t.Fatalf("got %v, want ErrMissingPayPhoneParams", err)
	}
	if _, err := c.ConfirmPayment(t.Context(), "1", ""); !errors.Is(err, ErrMissingPayPhoneParams) {
		t.Fatalf("got %v, want ErrMissingPayPhoneParams", err)
	}
}

func TestConfirmPayment_InvalidTransactionID(t *testing.T) {
	c := newTestClient(&config.Cfg{})

	_, err := c.ConfirmPayment(t.Context(), "not-a-number", "ctx-1")
	if !errors.Is(err, ErrInvalidTransactionID) {
		t.Fatalf("got %v, want ErrInvalidTransactionID", err)
	}
}

func TestConfirmPayment_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"boom"}`))
	}))
	defer ts.Close()

	restore := setConfirmAPIURLForTest(t, ts.URL)
	defer restore()

	c := newTestClient(&config.Cfg{Payphone: config.Payphone{Token: "ph-token"}})

	_, err := c.ConfirmPayment(t.Context(), "1", "ctx-1")
	if !errors.Is(err, ErrPayPhoneConfirmationAPI) {
		t.Fatalf("got %v, want ErrPayPhoneConfirmationAPI", err)
	}
}

func TestCancelPayment_IsNoOp(t *testing.T) {
	c := newTestClient(&config.Cfg{})

	if err := c.CancelPayment(t.Context(), "1", "ctx-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// setConfirmAPIURLForTest swaps payPhoneConfirmAPIURL for the duration of
// a test, returning a func that restores the real PayPhone URL.
func setConfirmAPIURLForTest(t *testing.T, url string) func() {
	t.Helper()
	original := payPhoneConfirmAPIURL
	payPhoneConfirmAPIURL = url
	return func() { payPhoneConfirmAPIURL = original }
}

// --- Inlined DTO-mapping logic (Task 17's core acceptance criteria) ---

func TestTransformProduct_UnitPriceFallback(t *testing.T) {
	// Price falls back to Subtotal/Quantity when UnitPrice (Price) is 0.
	p := domain.BasketProduct{Name: "Widget", Subtotal: 30, Quantity: 3}
	got := transformProduct(p, 0.1, "Acme")
	if got.Price != 10 {
		t.Errorf("Price = %v, want 10", got.Price)
	}
	if got.Tax != 1 {
		t.Errorf("Tax = %v, want 1", got.Tax)
	}
}

func TestTransformProduct_DescriptionFallback(t *testing.T) {
	p := domain.BasketProduct{Name: "Widget", Price: 5, Quantity: 1}
	got := transformProduct(p, 0, "Acme")
	if got.Description != "Product from Acme" {
		t.Errorf("Description = %q", got.Description)
	}
}

func TestTransformProduct_DescriptionTruncated(t *testing.T) {
	long := make([]byte, 300)
	for i := range long {
		long[i] = 'a'
	}
	p := domain.BasketProduct{Name: "Widget", Description: string(long), Price: 5, Quantity: 1}
	got := transformProduct(p, 0, "Acme")
	if len(got.Description) != descriptionMaxLength {
		t.Fatalf("len(Description) = %d, want %d", len(got.Description), descriptionMaxLength)
	}
	if got.Description[descriptionMaxLength-3:] != "..." {
		t.Errorf("Description doesn't end in ...: %q", got.Description[descriptionMaxLength-3:])
	}
}

func TestGenerateReference_Empty(t *testing.T) {
	got := generateReference("order-1", nil)
	if got != "Order order-1" {
		t.Errorf("got %q, want %q", got, "Order order-1")
	}
}

func TestGenerateReference_JoinsNames(t *testing.T) {
	got := generateReference("order-1", []string{"Widget", "Gadget"})
	if got != "Widget, Gadget" {
		t.Errorf("got %q", got)
	}
}

func TestGenerateReference_TruncatesManyNames(t *testing.T) {
	names := make([]string, 20)
	for i := range names {
		names[i] = "Product Name That Is Fairly Long"
	}
	got := generateReference("order-1", names)
	if len(got) > referenceMaxLength {
		t.Fatalf("reference too long: %d chars: %q", len(got), got)
	}
	if !containsSuffix(got, "more items") {
		t.Errorf("expected truncated reference to mention remaining items, got %q", got)
	}
}

func containsSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

func TestValidatePayPhoneOrder(t *testing.T) {
	tests := []struct {
		name    string
		order   payPhoneOrderRequest
		wantErr bool
	}{
		{
			name:    "missing client transaction id",
			order:   payPhoneOrderRequest{Products: []payPhoneProduct{{Name: "x", Price: 1, Quantity: 1}}, Amount: 1},
			wantErr: true,
		},
		{
			name:    "no products",
			order:   payPhoneOrderRequest{ClientTransactionID: "ctx-1", Amount: 1},
			wantErr: true,
		},
		{
			name:    "zero amount",
			order:   payPhoneOrderRequest{ClientTransactionID: "ctx-1", Products: []payPhoneProduct{{Name: "x", Price: 1, Quantity: 1}}},
			wantErr: true,
		},
		{
			name:    "invalid product",
			order:   payPhoneOrderRequest{ClientTransactionID: "ctx-1", Amount: 1, Products: []payPhoneProduct{{Name: "", Price: 1, Quantity: 1}}},
			wantErr: true,
		},
		{
			name:    "valid",
			order:   payPhoneOrderRequest{ClientTransactionID: "ctx-1", Amount: 1, Products: []payPhoneProduct{{Name: "x", Price: 1, Quantity: 1}}},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePayPhoneOrder(tt.order)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validatePayPhoneOrder() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
