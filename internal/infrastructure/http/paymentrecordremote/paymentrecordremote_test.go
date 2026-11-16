package paymentrecordremote_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/v8tix/beecore-eda/v2/config"

	"github.com/v8tix/beecore-store-v2/internal/infrastructure/http/paymentrecordremote"
)

func newTestClient(paymentsURL string) *paymentrecordremote.Client {
	return paymentrecordremote.NewClient(&config.Cfg{
		Web: config.Web{HTTPClient: http.DefaultClient},
		Integration: config.Integration{
			V1: config.IntegrationV1{PaymentsURL: paymentsURL},
		},
	})
}

func TestCreatePayment_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/payments" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer admin-tok" {
			t.Errorf("unexpected auth header: %s", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"payment":{"payment_id":"pay-1"}}`))
	}))
	defer ts.Close()

	err := newTestClient(ts.URL).CreatePayment(
		context.Background(), "pay-1", "user-1", "payphone", "PENDING", "{}", "", 14.50, "admin-tok",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreatePayment_DownstreamErrorPropagates(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"validation failed"}`))
	}))
	defer ts.Close()

	err := newTestClient(ts.URL).CreatePayment(
		context.Background(), "pay-1", "user-1", "payphone", "PENDING", "{}", "", 14.50, "admin-tok",
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFindPayment_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/payments/pay-1" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"payment":{"payment_id":"pay-1","user_id":"user-1","status":"PENDING","request":"{}"}}`))
	}))
	defer ts.Close()

	payment, err := newTestClient(ts.URL).FindPayment(context.Background(), "pay-1", "admin-tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if payment.ID != "pay-1" || payment.Status != "PENDING" {
		t.Fatalf("got %+v", payment)
	}
}

func TestFindPayment_NotFoundPropagates(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"payment not found"}`))
	}))
	defer ts.Close()

	_, err := newTestClient(ts.URL).FindPayment(context.Background(), "missing", "admin-tok")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestUpdatePayment_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/payments/pay-1" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	err := newTestClient(ts.URL).UpdatePayment(context.Background(), "pay-1", "CONFIRMED", "{}", "{}", "admin-tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdatePayment_DownstreamErrorPropagates(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer ts.Close()

	err := newTestClient(ts.URL).UpdatePayment(context.Background(), "pay-1", "CONFIRMED", "{}", "{}", "admin-tok")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
