package userremote_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/v8tix/beecore-eda/config"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/infrastructure/http/userremote"
)

func newTestClient(authURL string) *userremote.Client {
	return userremote.NewClient(&config.Cfg{
		Web: config.Web{HTTPClient: http.DefaultClient},
		Integration: config.Integration{
			V1: config.IntegrationV1{AuthURL: authURL},
		},
	})
}

func TestUpdateUser_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/users/u1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("unexpected auth header: %s", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	err := newTestClient(ts.URL).UpdateUser(context.Background(), "u1", "0102030405", "Jane", "Doe", "1990-01-01", "+593999999999", "", "", "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestUpdateUser_NotFoundTranslatesToSentinel proves the source
// updateUserWizard handler's 404 special-case is preserved as a domain
// sentinel instead of a raw kawa.ErrInvalidHTTPStatus.
func TestUpdateUser_NotFoundTranslatesToSentinel(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"user not found"}`))
	}))
	defer ts.Close()

	err := newTestClient(ts.URL).UpdateUser(context.Background(), "missing", "0102030405", "Jane", "Doe", "1990-01-01", "+593999999999", "", "", "tok")
	if !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("got %v, want domain.ErrUserNotFound", err)
	}
}

func TestUpdateUser_OtherErrorPropagatesGeneric(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"something went wrong"}`))
	}))
	defer ts.Close()

	err := newTestClient(ts.URL).UpdateUser(context.Background(), "u1", "0102030405", "Jane", "Doe", "1990-01-01", "+593999999999", "", "", "tok")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if errors.Is(err, domain.ErrUserNotFound) {
		t.Fatal("did not expect domain.ErrUserNotFound sentinel for a non-404 error")
	}
}
