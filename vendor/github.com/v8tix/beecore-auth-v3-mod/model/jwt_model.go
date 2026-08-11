package model

import "github.com/golang-jwt/jwt/v4"

type CustomClaims struct {
	UserID string   `json:"user_id"`
	Roles  []string `json:"roles"`
	jwt.RegisteredClaims
}
