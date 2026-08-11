package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
)

func TestContextGetAuthenticatedUser_NoneSet(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	if got := contextGetAuthenticatedUser(r); got != nil {
		t.Fatalf("got %+v, want nil for a request with no authenticated user set", got)
	}
}

func TestContextSetAndGetAuthenticatedUser(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	user := &domain.User{ID: "u1", Email: "a@b.com"}

	r = contextSetAuthenticatedUser(r, user)

	got := contextGetAuthenticatedUser(r)
	if got == nil {
		t.Fatal("got nil, want the authenticated user set above")
	}
	if got.ID != user.ID || got.Email != user.Email {
		t.Fatalf("got %+v, want %+v", got, user)
	}
}
