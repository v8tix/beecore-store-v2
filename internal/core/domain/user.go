package domain

import "time"

// User represents a storefront user account as exchanged with the
// downstream beecore-customers user service (see
// internal/business/core/repository/user_repository.go in the source
// repo). Optional fields are plain strings rather than pointers: every
// current call site either receives them already-resolved from a decoded
// wire response or builds them from validated form/request values, never
// from a raw nullable source.
type User struct {
	ID        string
	RoleID    string
	DNI       string
	FirstName string
	LastName  string
	Email     string
	Birthday  string
	Genre     string
	Phone     string
	ImgURL    string
	Website   string
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}
