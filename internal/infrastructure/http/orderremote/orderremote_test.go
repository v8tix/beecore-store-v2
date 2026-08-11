package orderremote_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/v8tix/beecore-eda/config"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/infrastructure/http/orderremote"
)

func newTestClient(ordersURL string) *orderremote.Client {
	return orderremote.NewClient(&config.Cfg{
		Web: config.Web{HTTPClient: http.DefaultClient},
		Integration: config.Integration{
			V1: config.IntegrationV1{OrdersURL: ordersURL},
		},
	})
}

func TestCreateOrder_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/orders" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("unexpected auth header: %s", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"order":{"order_id":"o1"}}`))
	}))
	defer ts.Close()

	req := domain.CreateOrderRequest{
		UserID:            "u1",
		PaymentID:         "p1",
		BasketID:          "b1",
		ShippingAddressID: "addr-1",
		FinancialItem: domain.FinancialLineItem{
			Store:    domain.Store{ID: "s1"},
			Products: []domain.BasketProduct{{ID: "prod-1", Quantity: 2}},
			Total:    11.5,
		},
	}

	orderID, err := newTestClient(ts.URL).CreateOrder(context.Background(), req, "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if orderID != "o1" {
		t.Fatalf("got %q, want %q", orderID, "o1")
	}
}

func TestCreateOrder_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer ts.Close()

	_, err := newTestClient(ts.URL).CreateOrder(context.Background(), domain.CreateOrderRequest{}, "tok")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetOrder_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/orders/o1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"order":{"order_id":"o1","user_id":"u1","payment_id":"p1","basket_id":"b1","status":"completed","shipping_address":{"country":"EC","city":"Quito"},"financial_item":{"store":{"id":"s1"},"products":[{"id":"prod-1","quantity":2}],"total":11.5},"updated_at":"2026-01-01T00:00:00Z"}}`))
	}))
	defer ts.Close()

	order, err := newTestClient(ts.URL).GetOrder(context.Background(), "o1", "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if order.ID != "o1" || order.UserID != "u1" || order.PaymentID != "p1" || order.BasketID != "b1" || order.Status != "completed" {
		t.Fatalf("unexpected order: %+v", order)
	}
	if order.ShippingAddress.Country != "EC" || order.ShippingAddress.City != "Quito" {
		t.Fatalf("unexpected shipping address: %+v", order.ShippingAddress)
	}
	if order.FinancialItem.Total != 11.5 || len(order.FinancialItem.Products) != 1 {
		t.Fatalf("unexpected financial item: %+v", order.FinancialItem)
	}
}

func TestGetOrder_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"order not found"}`))
	}))
	defer ts.Close()

	_, err := newTestClient(ts.URL).GetOrder(context.Background(), "missing", "tok")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetOrdersByUserID_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/orders/all/users/u1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"orders":[{"order_id":"o1","status":"completed"},{"order_id":"o2","status":"pending"}]}`))
	}))
	defer ts.Close()

	orders, err := newTestClient(ts.URL).GetOrdersByUserID(context.Background(), "u1", "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(orders) != 2 || orders[0].ID != "o1" || orders[1].ID != "o2" {
		t.Fatalf("unexpected orders: %+v", orders)
	}
}

func TestGetOrdersByUserID_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer ts.Close()

	_, err := newTestClient(ts.URL).GetOrdersByUserID(context.Background(), "u1", "tok")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// TestGetOrdersByUserID_UsesSlowProfileTimeout proves GetOrdersByUserID
// preserves the source repo's "slow" HTTP client profile override (10s
// default timeout) instead of the "fast" profile (1s default timeout)
// every other method in this package uses: a downstream response slower
// than the fast profile's timeout, but within the slow profile's, must
// still succeed.
func TestGetOrdersByUserID_UsesSlowProfileTimeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1200 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"orders":[{"order_id":"o1"}]}`))
	}))
	defer ts.Close()

	orders, err := newTestClient(ts.URL).GetOrdersByUserID(context.Background(), "u1", "tok")
	if err != nil {
		t.Fatalf("unexpected error (slow profile should tolerate a >1s response): %v", err)
	}
	if len(orders) != 1 || orders[0].ID != "o1" {
		t.Fatalf("unexpected orders: %+v", orders)
	}
}

func TestCancelOrder_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/orders/o1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	err := newTestClient(ts.URL).CancelOrder(context.Background(), "o1", "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCancelOrder_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer ts.Close()

	err := newTestClient(ts.URL).CancelOrder(context.Background(), "o1", "tok")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCompleteOrder_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/orders/o1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	err := newTestClient(ts.URL).CompleteOrder(context.Background(), "o1", "inv-1", "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCompleteOrder_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer ts.Close()

	err := newTestClient(ts.URL).CompleteOrder(context.Background(), "o1", "inv-1", "tok")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestReadyOrder_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/orders/o1/ready" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	err := newTestClient(ts.URL).ReadyOrder(context.Background(), "o1", "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReadyOrder_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer ts.Close()

	err := newTestClient(ts.URL).ReadyOrder(context.Background(), "o1", "tok")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
