package basketremote_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/v8tix/beecore-eda/config"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/infrastructure/http/basketremote"
)

func newTestClient(basketsURL string) *basketremote.Client {
	return basketremote.NewClient(&config.Cfg{
		Web: config.Web{HTTPClient: http.DefaultClient},
		Integration: config.Integration{
			V1: config.IntegrationV1{BasketsURL: basketsURL},
		},
	})
}

func TestCreateBasket_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/baskets" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("unexpected auth header: %s", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"basket":{"basket_id":"b1"}}`))
	}))
	defer ts.Close()

	id, err := newTestClient(ts.URL).CreateBasket(context.Background(), "u1", "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "b1" {
		t.Fatalf("got %q, want %q", id, "b1")
	}
}

func TestCreateBasket_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer ts.Close()

	_, err := newTestClient(ts.URL).CreateBasket(context.Background(), "u1", "tok")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFindBasket_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/baskets/b1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"basket":{"user_id":"u1","payment_id":"p1","status":"open","items":[{"store":{"id":"s1","name":"Store A"},"product":{"id":"prod-1","name":"Widget","quantity":2}}]}}`))
	}))
	defer ts.Close()

	basket, err := newTestClient(ts.URL).FindBasket(context.Background(), "b1", "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if basket.ID != "b1" || basket.UserID != "u1" || basket.PaymentID != "p1" || basket.Status != "open" {
		t.Fatalf("unexpected basket: %+v", basket)
	}
	if len(basket.Items) != 1 || basket.Items[0].Store.ID != "s1" || basket.Items[0].Product.ID != "prod-1" || basket.Items[0].Product.Quantity != 2 {
		t.Fatalf("unexpected basket items: %+v", basket.Items)
	}
}

func TestFindBasket_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"basket not found"}`))
	}))
	defer ts.Close()

	_, err := newTestClient(ts.URL).FindBasket(context.Background(), "missing", "tok")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestBasketAddItem_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/baskets/b1/addItem" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	err := newTestClient(ts.URL).BasketAddItem(context.Background(), "b1", "prod-1", 1, "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestBasketAddItem_Conflict proves a downstream 409 (the source repo's
// "basket may already be checked out" case) translates to
// domain.ErrBasketCheckedOut rather than a generic error.
func TestBasketAddItem_Conflict(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"basket already checked out"}`))
	}))
	defer ts.Close()

	err := newTestClient(ts.URL).BasketAddItem(context.Background(), "b1", "prod-1", 1, "tok")
	if !errors.Is(err, domain.ErrBasketCheckedOut) {
		t.Fatalf("got %v, want domain.ErrBasketCheckedOut", err)
	}
}

func TestBasketAddItem_GenericError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer ts.Close()

	err := newTestClient(ts.URL).BasketAddItem(context.Background(), "b1", "prod-1", 1, "tok")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if errors.Is(err, domain.ErrBasketCheckedOut) {
		t.Fatal("did not expect domain.ErrBasketCheckedOut for a non-409 error")
	}
}

func TestBasketRemoveItem_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/baskets/b1/removeItem" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	err := newTestClient(ts.URL).BasketRemoveItem(context.Background(), "b1", "prod-1", 1, "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBasketRemoveItem_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer ts.Close()

	err := newTestClient(ts.URL).BasketRemoveItem(context.Background(), "b1", "prod-1", 1, "tok")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCheckoutBasket_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/baskets/b1/checkout" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	err := newTestClient(ts.URL).CheckoutBasket(context.Background(), "b1", "pay-1", "addr-1", "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCancelBasket_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/baskets/b1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	err := newTestClient(ts.URL).CancelBasket(context.Background(), "b1", "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFindBasketByUserID_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/baskets/opened/users/u1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"basket":{"basket_id":"b1"}}`))
	}))
	defer ts.Close()

	id, err := newTestClient(ts.URL).FindBasketByUserID(context.Background(), "u1", "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "b1" {
		t.Fatalf("got %q, want %q", id, "b1")
	}
}

func TestFindBasketByUserID_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"basket not found"}`))
	}))
	defer ts.Close()

	_, err := newTestClient(ts.URL).FindBasketByUserID(context.Background(), "u1", "tok")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestComputeFinancials_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/baskets/b1/financials" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"financial":{"financial_items":[{"store":{"id":"s1"},"products":[{"id":"prod-1","quantity":2}],"shipping_amount":1.5,"subtotal":10,"total":11.5}],"subtotal":10,"total_shipping":1.5,"total_taxes":0,"total":11.5}}`))
	}))
	defer ts.Close()

	financials, err := newTestClient(ts.URL).ComputeFinancials(context.Background(), "b1", "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if financials.Total != 11.5 || financials.Subtotal != 10 || financials.TotalShippingAmount != 1.5 {
		t.Fatalf("unexpected financials: %+v", financials)
	}
	if len(financials.Items) != 1 || financials.Items[0].Store.ID != "s1" || len(financials.Items[0].Products) != 1 {
		t.Fatalf("unexpected financial items: %+v", financials.Items)
	}
}

func TestComputeFinancials_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer ts.Close()

	_, err := newTestClient(ts.URL).ComputeFinancials(context.Background(), "b1", "tok")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
