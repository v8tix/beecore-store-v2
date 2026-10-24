package users

import "github.com/v8tix/beecore-http/validator"

type (
	// GetTokenV1Req is the request to obtain an auth token for a user.
	GetTokenV1Req struct {
		Email    string `json:"email,omitempty"`
		Password string `json:"password,omitempty"`
	}

	// GetTokenV1Res is the response to GetTokenV1Req.
	GetTokenV1Res struct {
		Token        string `json:"token,omitempty"`
		RefreshToken string `json:"refresh_token,omitempty"`
	}

	// GetTokenEnvV1Res envelopes GetTokenV1Res.
	GetTokenEnvV1Res struct {
		Auth GetTokenV1Res `json:"auth,omitempty"`
	}
)

// Req marks GetTokenV1Req as a request payload.
func (g GetTokenV1Req) Req() {}

// Validate checks that GetTokenV1Req's fields are present.
func (g GetTokenV1Req) Validate(v *validator.Validator) {
	v.Check(len(g.Email) != 0, "email", "must be provided")
	v.Check(len(g.Password) != 0, "password", "must be provided")
}

// Res marks GetTokenEnvV1Res as a response payload.
func (g GetTokenEnvV1Res) Res() {}
