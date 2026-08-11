package session_test

import (
	"testing"
	"time"

	"github.com/v8tix/beecore-eda/config"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/infrastructure/redis/session"
)

func TestStore_SaveThenLoad_RoundTripsBlob(t *testing.T) {
	store := session.NewStore(config.NewInMemoryTokenCache())

	sess := domain.Session{
		UserAccessToken:     "user-jwt",
		UserAccessExpiry:    time.Now().Add(time.Hour).UTC().Truncate(time.Second),
		RefreshToken:        "refresh-opaque",
		UserID:              "user-1",
		UserDNI:             "12345678",
		UserShippingAddress: "addr-1",
	}

	if err := store.Save(t.Context(), "session-id-1", sess, time.Hour); err != nil {
		t.Fatalf("unexpected error saving: %v", err)
	}

	got, err := store.Load(t.Context(), "session-id-1")
	if err != nil {
		t.Fatalf("unexpected error loading: %v", err)
	}
	if got != sess {
		t.Fatalf("got %+v, want %+v", got, sess)
	}
}

func TestStore_Load_MissingSessionReturnsErrNotFound(t *testing.T) {
	store := session.NewStore(config.NewInMemoryTokenCache())

	_, err := store.Load(t.Context(), "does-not-exist")
	if err != session.ErrNotFound {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

func TestStore_Delete_RemovesSession(t *testing.T) {
	store := session.NewStore(config.NewInMemoryTokenCache())
	sess := domain.Session{UserID: "user-1"}

	if err := store.Save(t.Context(), "session-id-1", sess, time.Hour); err != nil {
		t.Fatalf("unexpected error saving: %v", err)
	}
	if err := store.Delete(t.Context(), "session-id-1"); err != nil {
		t.Fatalf("unexpected error deleting: %v", err)
	}

	_, err := store.Load(t.Context(), "session-id-1")
	if err != session.ErrNotFound {
		t.Fatalf("got %v, want ErrNotFound after delete", err)
	}
}
