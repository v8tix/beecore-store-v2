package authremote_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/v8tix/beecore-eda/config"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/infrastructure/http/authremote"
)

func newTestClient(authURL string) *authremote.Client {
	return authremote.NewClient(&config.Cfg{
		Web: config.Web{HTTPClient: http.DefaultClient},
		Integration: config.Integration{
			V1: config.IntegrationV1{AuthURL: authURL},
		},
	})
}

func TestFindUserByEmailAndPassword_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/search/verify-credentials" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("unexpected auth header: %s", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user":{"id":"u1","email":"a@b.com"}}`))
	}))
	defer ts.Close()

	user, err := newTestClient(ts.URL).FindUserByEmailAndPassword(context.Background(), "a@b.com", "pw", "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ID != "u1" {
		t.Fatalf("got ID %q, want %q", user.ID, "u1")
	}
}

func TestFindUserByEmailAndPassword_InvalidPassword(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid password"}`))
	}))
	defer ts.Close()

	_, err := newTestClient(ts.URL).FindUserByEmailAndPassword(context.Background(), "a@b.com", "wrong", "tok")
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("got %v, want domain.ErrInvalidCredentials", err)
	}
}

func TestFindUserByEmailAndPassword_UserNotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"user not found"}`))
	}))
	defer ts.Close()

	_, err := newTestClient(ts.URL).FindUserByEmailAndPassword(context.Background(), "missing@b.com", "pw", "tok")
	if !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("got %v, want domain.ErrUserNotFound", err)
	}
}

func TestFindUserByEmail_NotFoundTranslatesToSentinel(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"user not found"}`))
	}))
	defer ts.Close()

	_, err := newTestClient(ts.URL).FindUserByEmail(context.Background(), "missing@b.com", "tok")
	if !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("got %v, want domain.ErrUserNotFound", err)
	}
}

func TestFindUserByEmail_FoundReturnsUser(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user":{"user_id":"u1"}}`))
	}))
	defer ts.Close()

	user, err := newTestClient(ts.URL).FindUserByEmail(context.Background(), "a@b.com", "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ID != "u1" {
		t.Fatalf("got ID %q, want %q", user.ID, "u1")
	}
}

func TestFindUserByID_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/u1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer admin-tok" {
			t.Errorf("unexpected auth header: %s", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user":{"id":"u1","email":"a@b.com","phone":"0999999999","dni":"1234567890"}}`))
	}))
	defer ts.Close()

	user, err := newTestClient(ts.URL).FindUserByID(context.Background(), "u1", "admin-tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ID != "u1" || user.Email != "a@b.com" || user.Phone != "0999999999" || user.DNI != "1234567890" {
		t.Fatalf("unexpected user: %+v", user)
	}
}

func TestFindUserByID_NotFoundTranslatesToSentinel(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"user not found"}`))
	}))
	defer ts.Close()

	_, err := newTestClient(ts.URL).FindUserByID(context.Background(), "missing", "admin-tok")
	if !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("got %v, want domain.ErrUserNotFound", err)
	}
}

func TestActivateUser_TokenMismatchTranslatesToSentinel(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("unexpected method: %s", r.Method)
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"error":"token does not match"}`))
	}))
	defer ts.Close()

	err := newTestClient(ts.URL).ActivateUser(context.Background(), "admin-tok", "u1", "bad-code")
	if !errors.Is(err, domain.ErrInvalidActivationToken) {
		t.Fatalf("got %v, want domain.ErrInvalidActivationToken", err)
	}
}

func TestActivateUser_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	if err := newTestClient(ts.URL).ActivateUser(context.Background(), "admin-tok", "u1", "good-code"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRegisterUser_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user":{"user_id":"new-user"}}`))
	}))
	defer ts.Close()

	userID, err := newTestClient(ts.URL).RegisterUser(context.Background(), "Jane", "Doe", "jane@b.com", "pw", "pw", "admin-tok", "role-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if userID != "new-user" {
		t.Fatalf("got %q, want %q", userID, "new-user")
	}
}

func TestFindUserTokenByEmailAndPassword_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/token" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"auth":{"token":"access-tok","refresh_token":"refresh-tok"}}`))
	}))
	defer ts.Close()

	accessToken, refreshToken, err := newTestClient(ts.URL).FindUserTokenByEmailAndPassword(context.Background(), "a@b.com", "pw", "admin-tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if accessToken != "access-tok" || refreshToken != "refresh-tok" {
		t.Fatalf("got (%q, %q), want (%q, %q)", accessToken, refreshToken, "access-tok", "refresh-tok")
	}
}

func TestUpdateUserPassword_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("unexpected method: %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	if err := newTestClient(ts.URL).UpdateUserPassword(context.Background(), "u1", "newpw", "newpw", "tok"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSendResetPasswdEmail_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	if err := newTestClient(ts.URL).SendResetPasswdEmail(context.Background(), "a@b.com", "tok"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFindTokenByEmailAndPassword_UsesProvidedTokenWhenSet(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer explicit-tok" {
			t.Errorf("unexpected auth header: %s", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"auth":{"token":"access-tok"}}`))
	}))
	defer ts.Close()

	accessToken, err := newTestClient(ts.URL).FindTokenByEmailAndPassword(context.Background(), "admin@b.com", "pw", "explicit-tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if accessToken != "access-tok" {
		t.Fatalf("got %q, want %q", accessToken, "access-tok")
	}
}

// TestFindTokenByEmailAndPassword_ResponseIncludesRefreshToken is a
// regression test for the strict-decode failure fixed in this repo's
// Foundation phase: beecore-customers' real /auth/token response always
// includes refresh_token, and beecore-http's decoder rejects unknown
// fields with DisallowUnknownFields. Before the fix, decoding this
// response into the old users.GetTokenEnvV1Res (which only declared
// Token) would fail hard; now that the shared type also declares
// RefreshToken, this must keep passing. Mirrors
// beecore-admin-v2/internal/infrastructure/http/authremote's test of the
// same name.
func TestFindTokenByEmailAndPassword_ResponseIncludesRefreshToken(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"auth":{"token":"access-tok","refresh_token":"refresh-tok"}}`))
	}))
	defer ts.Close()

	accessToken, err := newTestClient(ts.URL).FindTokenByEmailAndPassword(context.Background(), "admin@b.com", "pw", "explicit-tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if accessToken != "access-tok" {
		t.Fatalf("got %q, want %q", accessToken, "access-tok")
	}
}

func TestRefreshUserToken_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/refresh" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"auth":{"token":"new-access-tok","refresh_token":"new-refresh-tok"}}`))
	}))
	defer ts.Close()

	accessToken, err := newTestClient(ts.URL).RefreshUserToken(context.Background(), "u1", "old-refresh-tok", "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if accessToken != "new-access-tok" {
		t.Fatalf("got %q, want %q", accessToken, "new-access-tok")
	}
}

func TestRefreshUserToken_RejectedSessionReturnsSentinel(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid refresh token"}`))
	}))
	defer ts.Close()

	_, err := newTestClient(ts.URL).RefreshUserToken(context.Background(), "u1", "expired-refresh-tok", "tok")
	if !errors.Is(err, domain.ErrSessionRejected) {
		t.Fatalf("got %v, want domain.ErrSessionRejected", err)
	}
}
