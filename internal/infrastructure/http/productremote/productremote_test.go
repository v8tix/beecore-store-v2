package productremote_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/v8tix/beecore-eda/config"

	"github.com/v8tix/beecore-store-v2/internal/infrastructure/http/productremote"
)

func newTestClient(storesURL string) *productremote.Client {
	return productremote.NewClient(&config.Cfg{
		Web: config.Web{HTTPClient: http.DefaultClient},
		Integration: config.Integration{
			V1: config.IntegrationV1{StoresURL: storesURL},
		},
	})
}

func TestFindProduct_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/stores/products/p1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("unexpected auth header: %s", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"product":{"id":"p1","store_id":"s1","name":"Widget","price":9.99,"quantity":5}}`))
	}))
	defer ts.Close()

	product, err := newTestClient(ts.URL).FindProduct(context.Background(), "p1", "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if product.ID != "p1" || product.StoreID != "s1" || product.Name != "Widget" || product.Price != 9.99 || product.Quantity != 5 {
		t.Fatalf("unexpected product: %+v", product)
	}
}

func TestFindProduct_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"product not found"}`))
	}))
	defer ts.Close()

	_, err := newTestClient(ts.URL).FindProduct(context.Background(), "missing", "tok")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFindPaginatedProducts_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/products/search/count":
			if got := r.URL.Query().Get("q"); got != "widget" {
				t.Errorf("unexpected q param on count call: %s", got)
			}
			_, _ = w.Write([]byte(`{"search":"widget","total":2}`))
		case "/products/search":
			if got := r.URL.Query().Get("q"); got != "widget" {
				t.Errorf("unexpected q param on search call: %s", got)
			}
			if got := r.URL.Query().Get("page"); got != "0" {
				t.Errorf("unexpected page param: %s", got)
			}
			_, _ = w.Write([]byte(`{"products":[{"id":"p1","name":"Widget A"},{"id":"p2","name":"Widget B"}]}`))
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer ts.Close()

	got, err := newTestClient(ts.URL).FindPaginatedProducts(context.Background(), "widget", 0, "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Total != 2 || len(got.Products) != 2 || got.Products[0].ID != "p1" || got.Products[1].ID != "p2" {
		t.Fatalf("unexpected page: %+v", got)
	}
}

func TestFindPaginatedProducts_CountError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer ts.Close()

	_, err := newTestClient(ts.URL).FindPaginatedProducts(context.Background(), "widget", 0, "tok")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFindPaginatedProducts_SearchError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/products/search/count":
			_, _ = w.Write([]byte(`{"search":"widget","total":0}`))
		default:
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"boom"}`))
		}
	}))
	defer ts.Close()

	_, err := newTestClient(ts.URL).FindPaginatedProducts(context.Background(), "widget", 0, "tok")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
