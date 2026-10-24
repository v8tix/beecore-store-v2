package users

import (
	"time"

	"github.com/v8tix/beecore-http/validator"
)

type (
	// User is the wire representation of a user account.
	User struct {
		ID        string    `json:"id,omitempty"`
		RoleID    string    `json:"role_id,omitempty"`
		DNI       *string   `json:"dni,omitempty"`
		FirstName string    `json:"first_name,omitempty"`
		LastName  string    `json:"last_name,omitempty"`
		Email     string    `json:"email,omitempty"`
		Birthday  *string   `json:"birthday,omitempty"`
		Genre     *string   `json:"genre,omitempty"`
		Phone     *string   `json:"phone,omitempty"`
		ImgURL    *string   `json:"img_url,omitempty"`
		Website   *string   `json:"website,omitempty"`
		Enabled   bool      `json:"enabled,omitempty"`
		CreatedAt time.Time `json:"created_at,omitempty"`
		UpdatedAt time.Time `json:"updated_at,omitempty"`
	}
)

type (
	// RegisterUserV1Req is the request to register a new user.
	RegisterUserV1Req struct {
		FirstName       string `json:"first_name,omitempty"`
		LastName        string `json:"last_name,omitempty"`
		Email           string `json:"email,omitempty"`
		Password        string `json:"password,omitempty"`
		ConfirmPassword string `json:"confirm_password,omitempty"`
		RoleID          string `json:"role_id,omitempty"`
	}

	// UpdateUserV1Req is the request to update a user's profile fields.
	UpdateUserV1Req struct {
		DNI       string `json:"dni,omitempty"`
		FirstName string `json:"first_name,omitempty"`
		LastName  string `json:"last_name,omitempty"`
		Birthday  string `json:"birthday,omitempty"`
		Genre     string `json:"genre,omitempty"`
		Phone     string `json:"phone,omitempty"`
		ImgURL    string `json:"img_url,omitempty"`
		Website   string `json:"website,omitempty"`
	}

	// GetUserByEmailV1Req is the request to look up a user by email.
	GetUserByEmailV1Req struct {
		Email string `json:"email,omitempty"`
	}

	// GetUserByEmailAndPasswordV1Req is the request to look up a user by email and password.
	GetUserByEmailAndPasswordV1Req struct {
		Email    string `json:"email,omitempty"`
		Password string `json:"password,omitempty"`
	}

	// ActivateUserV1Req is the request to activate a user account via its activation token.
	ActivateUserV1Req struct {
		ID    string `json:"user_id,omitempty"`
		Token string `json:"activation_token,omitempty"`
	}

	// EnableUserV1Req is the request to enable a user account.
	EnableUserV1Req struct {
		ID string `json:"user_id,omitempty"`
	}

	// DisableUserV1Req is the request to disable a user account.
	DisableUserV1Req struct {
		ID string `json:"user_id,omitempty"`
	}

	// GetUserByEmailV1Res is the response to GetUserByEmailV1Req.
	GetUserByEmailV1Res struct {
		ID string `json:"user_id,omitempty"`
	}

	// RegisterUserV1Res is the response to RegisterUserV1Req.
	RegisterUserV1Res struct {
		ID string `json:"user_id,omitempty"`
	}

	// RegisterUserEnvV1Res envelopes RegisterUserV1Res.
	RegisterUserEnvV1Res struct {
		User RegisterUserV1Res `json:"user,omitempty"`
	}

	// GetUserByStoreIDV1Res is the response to a lookup of a user by store ID.
	GetUserByStoreIDV1Res struct {
		ID string `json:"user_id,omitempty"`
	}

	// GetUserByStoreIDEnvV1Res envelopes GetUserByStoreIDV1Res.
	GetUserByStoreIDEnvV1Res struct {
		User GetUserByStoreIDV1Res `json:"user,omitempty"`
	}

	// GetUserByEmailEnvV1Res envelopes GetUserByEmailV1Res.
	GetUserByEmailEnvV1Res struct {
		User GetUserByEmailV1Res `json:"user,omitempty"`
	}

	// UserV1EnvV1Res envelopes a full User.
	UserV1EnvV1Res struct {
		User User `json:"user,omitempty"`
	}

	// ResetUserPasswordV1Req is the request to start a password reset for a user.
	ResetUserPasswordV1Req struct {
		Email string `json:"email,omitempty"`
	}

	// UpdateUserPasswordV1Req is the request to set a user's new password.
	UpdateUserPasswordV1Req struct {
		Password        string `json:"password,omitempty"`
		ConfirmPassword string `json:"confirm_password,omitempty"`
	}
)

// Req marks ActivateUserV1Req as a request payload.
func (a ActivateUserV1Req) Req() {}

// Validate checks that ActivateUserV1Req's fields are present.
func (a ActivateUserV1Req) Validate(v *validator.Validator) {
	v.Check(len(a.ID) != 0, "user_id", "must be provided")
	v.Check(len(a.Token) != 0, "activation_token", "must be provided")
}

// Req marks RegisterUserV1Req as a request payload.
func (r RegisterUserV1Req) Req() {}

// Validate checks that RegisterUserV1Req's fields are present and consistent.
func (r RegisterUserV1Req) Validate(v *validator.Validator) {
	v.Check(len(r.FirstName) != 0, "first_name", "must be provided")
	v.Check(len(r.LastName) != 0, "last_name", "must be provided")
	v.Check(len(r.Email) != 0, "email", "must be provided")
	v.Check(len(r.Password) != 0, "password", "must be provided")
	v.Check(len(r.ConfirmPassword) != 0, "confirm_password", "must be provided")
	v.Check(len(r.RoleID) != 0, "role_id", "must be provided")
	v.Check(r.Password == r.ConfirmPassword, "confirm_password", "confirm_password does not match the password")
}

// Req marks EnableUserV1Req as a request payload.
func (e EnableUserV1Req) Req() {}

// Validate checks that EnableUserV1Req's fields are present.
func (e EnableUserV1Req) Validate(v *validator.Validator) {
	v.Check(len(e.ID) != 0, "user_id", "must be provided")
}

// Req marks DisableUserV1Req as a request payload.
func (d DisableUserV1Req) Req() {}

// Validate checks that DisableUserV1Req's fields are present.
func (d DisableUserV1Req) Validate(v *validator.Validator) {
	v.Check(len(d.ID) != 0, "user_id", "must be provided")
}

// Res marks RegisterUserEnvV1Res as a response payload.
func (r RegisterUserEnvV1Res) Res() {}

// Req marks GetUserByEmailV1Req as a request payload.
func (g GetUserByEmailV1Req) Req() {}

// Validate checks that GetUserByEmailV1Req's fields are present.
func (g GetUserByEmailV1Req) Validate(v *validator.Validator) {
	v.Check(len(g.Email) != 0, "email", "must be provided")
}

// Req marks GetUserByEmailAndPasswordV1Req as a request payload.
func (g GetUserByEmailAndPasswordV1Req) Req() {}

// Validate checks that GetUserByEmailAndPasswordV1Req's fields are present.
func (g GetUserByEmailAndPasswordV1Req) Validate(v *validator.Validator) {
	v.Check(len(g.Email) != 0, "email", "must be provided")
	v.Check(len(g.Password) != 0, "password", "must be provided")
}

// Res marks GetUserByEmailEnvV1Res as a response payload.
func (g GetUserByEmailEnvV1Res) Res() {}

// Res marks UserV1EnvV1Res as a response payload.
func (g UserV1EnvV1Res) Res() {}

// Req marks UpdateUserV1Req as a request payload.
func (u UpdateUserV1Req) Req() {}

// Validate checks that UpdateUserV1Req's fields are present.
func (u UpdateUserV1Req) Validate(v *validator.Validator) {
	v.Check(len(u.DNI) != 0, "dni", "must be provided")
	v.Check(len(u.FirstName) != 0, "first_name", "must be provided")
	v.Check(len(u.LastName) != 0, "last_name", "must be provided")
	v.Check(len(u.Phone) != 0, "phone", "must be provided")
}

// Req marks ResetUserPasswordV1Req as a request payload.
func (r ResetUserPasswordV1Req) Req() {}

// Validate checks that ResetUserPasswordV1Req's fields are present.
func (r ResetUserPasswordV1Req) Validate(v *validator.Validator) {
	v.Check(len(r.Email) != 0, "email", "must be provided")
}

// Req marks UpdateUserPasswordV1Req as a request payload.
func (u UpdateUserPasswordV1Req) Req() {}

// Validate checks that UpdateUserPasswordV1Req's fields are present and consistent.
func (u UpdateUserPasswordV1Req) Validate(v *validator.Validator) {
	v.Check(len(u.Password) != 0, "password", "must be provided")
	v.Check(len(u.ConfirmPassword) != 0, "confirm_password", "must be provided")
	v.Check(u.Password == u.ConfirmPassword, "confirm_password", "confirm_password does not match the password")
}

// Res marks GetUserByStoreIDEnvV1Res as a response payload.
func (g GetUserByStoreIDEnvV1Res) Res() {}
