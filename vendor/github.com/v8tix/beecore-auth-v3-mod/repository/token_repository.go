package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/v8tix/beecore-auth-v3-mod/model"
)

var (
	ErrInsertToken   = errors.New("could not insert token")
	ErrUpdateToken   = errors.New("could not update token")
	ErrTokenNotFound = errors.New("token not found")
	ErrTokenScope    = errors.New("token scope not found")

	ActivationToken     = TokenScope("ACTIVATION")
	AuthenticationToken = TokenScope("AUTHENTICATION")
	RefreshToken        = TokenScope("REFRESH")
)

type TokenScope string

func (r Repository) FindTokenByUserIDAndScope(ctx context.Context, userID string, scope TokenScope) (*model.Token, error) {
	if scope != AuthenticationToken && scope != ActivationToken && scope != RefreshToken {
		return nil, ErrTokenScope
	}

	var token model.Token

	err := r.db.QueryRow(ctx, `
		SELECT 
		    id, 
		    user_id, 
		    token,  
		    scope, 
		    expiry, 
		    is_revoked,
		    created_at, 
		    updated_at
		FROM users.tokens
		WHERE user_id = $1 AND scope = $2`, userID, scope).
		Scan(
			&token.ID,
			&token.UserID,
			&token.Value,
			&token.Scope,
			&token.ExpiredAt,
			&token.IsRevoked,
			&token.CreatedAt,
			&token.UpdatedAt,
		)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%w: %v", ErrTokenNotFound, err)
		}
		return nil, fmt.Errorf("%w: %v", ErrQuery, err)
	}

	return &token, nil
}

func (r Repository) insertToken(ctx context.Context, token *model.Token) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO users.tokens (
			id, 
			user_id, 
			token, 
			scope, 
			expiry
		) VALUES ($1, $2, $3, $4, $5)
	`, token.ID, token.UserID, token.Value, token.Scope, token.ExpiredAt)

	if err != nil {
		return fmt.Errorf("%w: %v", ErrInsertToken, err)
	}

	return nil
}

func (r Repository) UpdateToken(ctx context.Context, token *model.Token) error {
	_, err := r.db.Exec(ctx, `
		UPDATE users.tokens
		SET
			token = $1,
			expiry = $2,
			is_revoked = $3,
			updated_at = NOW()
		WHERE user_id = $4 AND scope = $5
	`, token.Value, token.ExpiredAt, token.IsRevoked, token.UserID, token.Scope)

	if err != nil {
		return fmt.Errorf("%w: %v", ErrUpdateToken, err)
	}

	return nil
}
