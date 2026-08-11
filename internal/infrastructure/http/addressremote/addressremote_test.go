package addressremote_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/v8tix/beecore-eda/config"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/infrastructure/http/addressremote"
)

func newTestClient(authURL string) *addressremote.Client {
	return addressremote.NewClient(&config.Cfg{
		Web: config.Web{HTTPClient: http.DefaultClient},
		Integration: config.Integration{
			V1: config.IntegrationV1{AuthURL: authURL},
		},
	})
}

func TestFindAddressesByUserID_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/u1/addresses" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("unexpected auth header: %s", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"addresses":[{"id":"addr-1","country":"EC"},{"id":"addr-2","country":"US"}]}`))
	}))
	defer ts.Close()

	addresses, err := newTestClient(ts.URL).FindAddressesByUserID(context.Background(), "u1", "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(addresses) != 2 || addresses[0].ID != "addr-1" || addresses[0].Country != "EC" {
		t.Fatalf("unexpected addresses: %+v", addresses)
	}
}

func TestFindAddressesByUserID_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer ts.Close()

	_, err := newTestClient(ts.URL).FindAddressesByUserID(context.Background(), "u1", "tok")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestHasAddresses_Found(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"addresses":[{"id":"addr-1"},{"id":"addr-2"}]}`))
	}))
	defer ts.Close()

	has, firstID, err := newTestClient(ts.URL).HasAddresses(context.Background(), "u1", "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !has || firstID != "addr-1" {
		t.Fatalf("got (%v, %q), want (true, %q)", has, firstID, "addr-1")
	}
}

// TestHasAddresses_DownstreamErrorIsSwallowed proves the source repo's
// HasAddresses quirk is preserved, unchanged, after relocating from
// AuthRemote: any downstream HTTP error with a JSON-parseable body — not
// just "addresses not found" — comes back as (false, "", nil), not an
// error.
func TestHasAddresses_DownstreamErrorIsSwallowed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"something else entirely"}`))
	}))
	defer ts.Close()

	has, firstID, err := newTestClient(ts.URL).HasAddresses(context.Background(), "u1", "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if has || firstID != "" {
		t.Fatalf("got (%v, %q), want (false, \"\")", has, firstID)
	}
}

func TestInsertAddress_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/users/u1/addresses" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"address":{"address_id":"new-addr"}}`))
	}))
	defer ts.Close()

	id, err := newTestClient(ts.URL).InsertAddress(context.Background(), "u1", domain.ShippingAddress, "EC", "Quito", "Pichincha", "170101", "Main St", "2nd St", "123", "+593999999999", "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "new-addr" {
		t.Fatalf("got %q, want %q", id, "new-addr")
	}
}

func TestInsertAddress_BadAddressType(t *testing.T) {
	client := newTestClient("http://unused.invalid")

	_, err := client.InsertAddress(context.Background(), "u1", domain.AddressType("Bogus"), "EC", "Quito", "Pichincha", "170101", "Main St", "2nd St", "123", "+593999999999", "tok")
	if !errors.Is(err, domain.ErrBadAddressType) {
		t.Fatalf("got %v, want domain.ErrBadAddressType", err)
	}
}

// TestInsertAddress_MaxAddressesReached proves the source repo's newAddress
// handler special-case (address_handler.go) is preserved as a domain
// sentinel: a downstream "maximum address length reached" error message
// translates to domain.ErrMaxAddressesReached.
func TestInsertAddress_MaxAddressesReached(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"error":"maximum address length reached"}`))
	}))
	defer ts.Close()

	_, err := newTestClient(ts.URL).InsertAddress(context.Background(), "u1", domain.ShippingAddress, "EC", "Quito", "Pichincha", "170101", "Main St", "2nd St", "123", "+593999999999", "tok")
	if !errors.Is(err, domain.ErrMaxAddressesReached) {
		t.Fatalf("got %v, want domain.ErrMaxAddressesReached", err)
	}
}

func TestUpdateAddress_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/users/u1/addresses/addr-1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	err := newTestClient(ts.URL).UpdateAddress(context.Background(), "addr-1", "u1", domain.ShippingAddress, "EC", "Quito", "Pichincha", "170101", "Main St", "2nd St", "123", "+593999999999", "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateAddress_BadAddressType(t *testing.T) {
	client := newTestClient("http://unused.invalid")

	err := client.UpdateAddress(context.Background(), "addr-1", "u1", domain.AddressType("Bogus"), "EC", "Quito", "Pichincha", "170101", "Main St", "2nd St", "123", "+593999999999", "tok")
	if !errors.Is(err, domain.ErrBadAddressType) {
		t.Fatalf("got %v, want domain.ErrBadAddressType", err)
	}
}
