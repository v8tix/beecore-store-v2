// Package session holds the Redis-backed adapter implementing
// resource.SessionStore. It is a near-verbatim port of
// internal/session/session.go from the source repo, retargeted to persist
// domain.Session instead of the source's package-local Blob type.
//
// Semantics resolved before this design (see beecore-store phase-1 spec
// Finding 1 and Decision 2):
//   - keys.AccessToken (old cookie key) was the shared ADMIN SERVICE-ACCOUNT
//     token — one credential for the whole app, not per-user state. It does
//     NOT belong in a per-session blob; callers fetch it fresh (and cached)
//     via app.repo.GetAdminToken() whenever they need to call
//     beecore-customers with admin privileges.
//   - keys.UserAccessToken (old cookie key) was the REAL logged-in user's
//     own signed JWT, obtained via beecore-customers' /auth/token with the
//     user's actual credentials. This now lives here, in domain.Session,
//     never in the cookie.
//   - Basket/payment/pagination/search session state (SessionBasketID,
//     SessionPaymentID, SessionSelfPage, etc.) deliberately stays in the
//     cookie's session.Values, unchanged — it isn't sensitive, and moving it
//     server-side wasn't part of this ticket's scope (Decision 2).
package session

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/v8tix/beecore-eda/config"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/core/port/resource"
)

var ErrNotFound = errors.New("session not found")

// Store persists and retrieves domain.Session records in a
// config.TokenCache, keyed by the opaque session ID that's the only thing
// the browser cookie holds for identity purposes.
type Store struct {
	cache config.TokenCache
}

var _ resource.SessionStore = Store{}

func NewStore(cache config.TokenCache) Store {
	return Store{cache: cache}
}

func (s Store) Save(ctx context.Context, sessionID string, sess domain.Session, ttl time.Duration) error {
	data, err := json.Marshal(sess)
	if err != nil {
		return err
	}

	return s.cache.Put(ctx, sessionID, string(data), ttl)
}

func (s Store) Load(ctx context.Context, sessionID string) (domain.Session, error) {
	var sess domain.Session

	value, _, ok, err := s.cache.Get(ctx, sessionID)
	if err != nil {
		return sess, err
	}
	if !ok {
		return sess, ErrNotFound
	}

	if err := json.Unmarshal([]byte(value), &sess); err != nil {
		return sess, err
	}

	return sess, nil
}

func (s Store) Delete(ctx context.Context, sessionID string) error {
	return s.cache.Remove(ctx, sessionID)
}
