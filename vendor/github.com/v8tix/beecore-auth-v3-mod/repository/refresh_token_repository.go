package repository

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/v8tix/beecore-auth-v3-mod/model"
)

var (
	ErrGenerateRefreshToken = errors.New("failed to generate refresh token")
	ErrRefreshTokenExpired  = errors.New("refresh token is expired")
	ErrRefreshTokenMismatch = errors.New("refresh token does not match")
)

// GenerateRefreshToken issues a new high-entropy opaque refresh token for
// userID and persists it under RefreshToken scope, replacing any existing
// one for that user. Unlike GenerateAuthenticationToken, this never involves
// KMS — the refresh token is not signed, only looked up server-side.
//
// A successful call always un-revokes the row: reaching this point means the
// password was just verified (see beecore-customers' GetToken), which is a
// genuine re-establishment of trust, unlike RefreshAuthenticationToken below.
func (r Repository) GenerateRefreshToken(userID string, ttl time.Duration) (string, error) {
	value, err := randomOpaqueToken()
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrGenerateRefreshToken, err)
	}

	if err := r.upsertToken(context.Background(), userID, value, string(RefreshToken), false, time.Now().UTC().Add(ttl)); err != nil {
		return "", err
	}

	return value, nil
}

// RefreshAuthenticationToken exchanges a valid, non-expired, non-revoked
// refresh token for a new short-lived KMS-signed access token — without the
// user's password, which is never retained after login. This is the only
// operation the refresh token authorizes.
//
// Unlike GenerateRefreshToken, this must NOT un-revoke a revoked
// AUTHENTICATION row: possession of a still-valid refresh token proves the
// session hasn't been revoked at the refresh-token level, but a separately
// revoked access token (e.g. a future "kill this token" action) must stay
// revoked — refreshing is not a re-authentication event the way login is.
func (r Repository) RefreshAuthenticationToken(userID, refreshTokenValue, app string, expirationDuration time.Duration) (string, error) {
	ctx := context.Background()

	refresh, err := r.FindTokenByUserIDAndScope(ctx, userID, RefreshToken)
	if err != nil {
		return "", err
	}

	if err := validateRefreshToken(refresh, refreshTokenValue, time.Now().UTC()); err != nil {
		return "", err
	}

	existingAccessToken, err := r.FindTokenByUserIDAndScope(ctx, userID, AuthenticationToken)
	if err != nil && !errors.Is(err, ErrTokenNotFound) {
		return "", err
	}
	if isRevoked(existingAccessToken) {
		return "", ErrTokenRevoked
	}

	user, err := r.findUserByID(ctx, userID)
	if err != nil {
		return "", err
	}

	if !user.Enabled {
		return "", ErrDisabledUser
	}

	newToken, err := r.issueAccessToken(ctx, user, app, expirationDuration)
	if err != nil {
		slog.Error("RefreshAuthenticationToken: Failed to generate token", "user_id", user.ID, "error", err)
		return "", err
	}

	if err := r.upsertToken(ctx, userID, newToken, string(AuthenticationToken), false, time.Now().UTC().Add(expirationDuration)); err != nil {
		return "", err
	}

	return newToken, nil
}

// isRevoked reports whether an existing token row is explicitly revoked. A
// nil row (none found yet) is not revoked — there's nothing to revoke.
func isRevoked(existing *model.Token) bool {
	return existing != nil && existing.IsRevoked
}

// validateRefreshToken checks a stored refresh token against the value
// presented by the caller — not revoked, not expired, and an exact
// (constant-time) match. Pulled out of RefreshAuthenticationToken so this
// logic is testable without a live Postgres connection.
func validateRefreshToken(stored *model.Token, presented string, now time.Time) error {
	if stored.IsRevoked {
		return ErrTokenRevoked
	}

	if now.After(stored.ExpiredAt) {
		return ErrRefreshTokenExpired
	}

	if subtle.ConstantTimeCompare([]byte(stored.Value), []byte(presented)) != 1 {
		return ErrRefreshTokenMismatch
	}

	return nil
}

// upsertToken atomically inserts a token row for (userID, scope), or updates
// it in place if one already exists — a single statement, so concurrent
// callers for the same (userID, scope) (e.g. two logins, or two /auth/refresh
// calls racing) can't lose an update or collide on the unique_user_token_type
// constraint the way a separate find-then-insert-or-update would.
func (r Repository) upsertToken(ctx context.Context, userID, value, scope string, revoked bool, expiredAt time.Time) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO users.tokens (id, user_id, token, scope, expiry, is_revoked)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, scope) DO UPDATE SET
			token = EXCLUDED.token,
			expiry = EXCLUDED.expiry,
			is_revoked = EXCLUDED.is_revoked,
			updated_at = NOW()
	`, uuid.New().String(), userID, value, scope, expiredAt, revoked)

	if err != nil {
		return fmt.Errorf("%w: %v", ErrUpdateToken, err)
	}

	return nil
}

func randomOpaqueToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}
