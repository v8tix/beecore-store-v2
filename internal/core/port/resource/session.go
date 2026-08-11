package resource

import (
	"context"
	"time"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
)

//go:generate mockery --name SessionStore

// SessionStore persists and retrieves a logged-in storefront user's
// server-side session record (domain.Session), keyed by the opaque
// session ID that's the only thing the browser cookie holds. It mirrors
// the methods internal/session.Store exposes in the source repo.
type SessionStore interface {
	Save(ctx context.Context, sessionID string, session domain.Session, ttl time.Duration) error
	Load(ctx context.Context, sessionID string) (domain.Session, error)
	Delete(ctx context.Context, sessionID string) error
}
