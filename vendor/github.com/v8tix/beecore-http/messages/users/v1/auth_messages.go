package users

import "github.com/v8tix/beecore-http/validator"

type (
	GetTokenV1Req struct {
		Email    string `json:"email,omitempty"`
		Password string `json:"password,omitempty"`
	}

	GetTokenV1Res struct {
		Token        string `json:"token,omitempty"`
		RefreshToken string `json:"refresh_token,omitempty"`
	}

	GetTokenEnvV1Res struct {
		Auth GetTokenV1Res `json:"auth,omitempty"`
	}
)

func (g GetTokenV1Req) Req() {}

func (g GetTokenV1Req) Validate(v *validator.Validator) {
	v.Check(len(g.Email) != 0, "email", "must be provided")
	v.Check(len(g.Password) != 0, "password", "must be provided")
}

func (g GetTokenEnvV1Res) Res() {}
