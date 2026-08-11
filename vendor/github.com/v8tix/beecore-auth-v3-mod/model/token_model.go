package model

import "time"

type Token struct {
	ID        string
	UserID    string
	Value     string
	Scope     string
	IsRevoked bool
	ExpiredAt time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewToken(ID string, userID string, value string, scope string, isRevoked bool, expiredAt time.Time) *Token {
	return &Token{ID: ID, UserID: userID, Value: value, Scope: scope, IsRevoked: isRevoked, ExpiredAt: expiredAt}
}
