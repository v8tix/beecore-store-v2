package users

import (
	"time"

	"github.com/v8tix/beecore-http/validator"
)

type (
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
	RegisterUserV1Req struct {
		FirstName       string `json:"first_name,omitempty"`
		LastName        string `json:"last_name,omitempty"`
		Email           string `json:"email,omitempty"`
		Password        string `json:"password,omitempty"`
		ConfirmPassword string `json:"confirm_password,omitempty"`
		RoleID          string `json:"role_id,omitempty"`
	}

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

	GetUserByEmailV1Req struct {
		Email string `json:"email,omitempty"`
	}

	GetUserByEmailAndPasswordV1Req struct {
		Email    string `json:"email,omitempty"`
		Password string `json:"password,omitempty"`
	}

	ActivateUserV1Req struct {
		ID    string `json:"user_id,omitempty"`
		Token string `json:"activation_token,omitempty"`
	}

	EnableUserV1Req struct {
		ID string `json:"user_id,omitempty"`
	}

	DisableUserV1Req struct {
		ID string `json:"user_id,omitempty"`
	}

	GetUserByEmailV1Res struct {
		ID string `json:"user_id,omitempty"`
	}

	RegisterUserV1Res struct {
		ID string `json:"user_id,omitempty"`
	}

	RegisterUserEnvV1Res struct {
		User RegisterUserV1Res `json:"user,omitempty"`
	}

	GetUserByStoreIDV1Res struct {
		ID string `json:"user_id,omitempty"`
	}

	GetUserByStoreIDEnvV1Res struct {
		User GetUserByStoreIDV1Res `json:"user,omitempty"`
	}
	GetUserByEmailEnvV1Res struct {
		User GetUserByEmailV1Res `json:"user,omitempty"`
	}
	UserV1EnvV1Res struct {
		User User `json:"user,omitempty"`
	}

	ResetUserPasswordV1Req struct {
		Email string `json:"email,omitempty"`
	}

	UpdateUserPasswordV1Req struct {
		Password        string `json:"password,omitempty"`
		ConfirmPassword string `json:"confirm_password,omitempty"`
	}
)

func (a ActivateUserV1Req) Req() {}

func (a ActivateUserV1Req) Validate(v *validator.Validator) {
	v.Check(len(a.ID) != 0, "user_id", "must be provided")
	v.Check(len(a.Token) != 0, "activation_token", "must be provided")
}

func (r RegisterUserV1Req) Req() {}

func (r RegisterUserV1Req) Validate(v *validator.Validator) {
	v.Check(len(r.FirstName) != 0, "first_name", "must be provided")
	v.Check(len(r.LastName) != 0, "last_name", "must be provided")
	v.Check(len(r.Email) != 0, "email", "must be provided")
	v.Check(len(r.Password) != 0, "password", "must be provided")
	v.Check(len(r.ConfirmPassword) != 0, "confirm_password", "must be provided")
	v.Check(len(r.RoleID) != 0, "role_id", "must be provided")
	v.Check(r.Password == r.ConfirmPassword, "confirm_password", "confirm_password does not match the password")
}

func (e EnableUserV1Req) Req() {}

func (e EnableUserV1Req) Validate(v *validator.Validator) {
	v.Check(len(e.ID) != 0, "user_id", "must be provided")
}

func (d DisableUserV1Req) Req() {}

func (d DisableUserV1Req) Validate(v *validator.Validator) {
	v.Check(len(d.ID) != 0, "user_id", "must be provided")
}

func (r RegisterUserEnvV1Res) Res() {}

func (g GetUserByEmailV1Req) Req() {}

func (g GetUserByEmailV1Req) Validate(v *validator.Validator) {
	v.Check(len(g.Email) != 0, "email", "must be provided")
}

func (g GetUserByEmailAndPasswordV1Req) Req() {}

func (g GetUserByEmailAndPasswordV1Req) Validate(v *validator.Validator) {
	v.Check(len(g.Email) != 0, "email", "must be provided")
	v.Check(len(g.Password) != 0, "password", "must be provided")
}

func (g GetUserByEmailEnvV1Res) Res() {}

func (g UserV1EnvV1Res) Res() {}

func (u UpdateUserV1Req) Req() {}

func (u UpdateUserV1Req) Validate(v *validator.Validator) {
	v.Check(len(u.DNI) != 0, "dni", "must be provided")
	v.Check(len(u.FirstName) != 0, "first_name", "must be provided")
	v.Check(len(u.LastName) != 0, "last_name", "must be provided")
	v.Check(len(u.Phone) != 0, "phone", "must be provided")
}

func (r ResetUserPasswordV1Req) Req() {}

func (r ResetUserPasswordV1Req) Validate(v *validator.Validator) {
	v.Check(len(r.Email) != 0, "email", "must be provided")
}

func (u UpdateUserPasswordV1Req) Req() {}

func (u UpdateUserPasswordV1Req) Validate(v *validator.Validator) {
	v.Check(len(u.Password) != 0, "password", "must be provided")
	v.Check(len(u.ConfirmPassword) != 0, "confirm_password", "must be provided")
	v.Check(u.Password == u.ConfirmPassword, "confirm_password", "confirm_password does not match the password")
}

func (g GetUserByStoreIDEnvV1Res) Res() {}
