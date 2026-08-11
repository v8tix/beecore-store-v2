package authremote

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	users "github.com/v8tix/beecore-http/messages/users/v1"
	"github.com/v8tix/kawa"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
)

// GetAdminToken returns the cached admin service-account token, mirroring
// BaseRepositoryImpl.GetAdminToken — which just forwards to
// AuthService.GetOrGenerateAuthenticationToken (embedded on *config.Cfg).
func (c *Client) GetAdminToken(ctx context.Context) (string, error) {
	return c.cfg.GetOrGenerateAuthenticationToken(ctx, c.cfg.Admin.Email, c.cfg.Admin.Password, c.cfg.App.Name)
}

// FindTokenByEmailAndPassword mirrors
// BaseRepositoryImpl.FindTokenByEmailAndPassword: POST auth/token,
// decoding into the shared (now-fixed) users.GetTokenEnvV1Res but
// returning only the access token — see AuthRemote's doc comment for why
// decoding into the shared type (rather than a local workaround) matters
// now that this repo's beecore-http replace points at the fixed checkout.
func (c *Client) FindTokenByEmailAndPassword(ctx context.Context, email, password, token string) (string, error) {
	var authHeader map[string]string

	if len(token) == 0 {
		adminToken, err := c.GetAdminToken(ctx)
		if err != nil {
			return "", err
		}

		authHeader = buildAuthHeader(adminToken)
	} else {
		authHeader = buildAuthHeader(token)
	}

	req := users.GetTokenV1Req{Email: email, Password: password}

	url := fmt.Sprintf("%s/%s", c.cfg.Integration.V1.AuthURL, "auth/token")
	call := kawa.NewCall[users.GetTokenV1Req, users.GetTokenEnvV1Res](c.cfg.Web.HTTPClient, kawa.Post, url).
		WithHeaders(authHeader).
		WithDeadline(c.deadline()).
		WithRetryPolicy(c.retryPolicy())

	env, err := call.DoWithRetry(ctx, &req)
	if err != nil {
		return "", translateHTTPError(err)
	}

	if env.Body == nil {
		return "", nil
	}

	return env.Body.Auth.Token, nil
}

// FindUserByEmail mirrors BaseRepositoryImpl.FindUserByEmail. err is
// domain.ErrUserNotFound on a downstream 404.
func (c *Client) FindUserByEmail(ctx context.Context, email, token string) (domain.User, error) {
	url := fmt.Sprintf("%s/%s", c.cfg.Integration.V1.AuthURL, "users/search/email")

	req := users.GetUserByEmailV1Req{Email: email}

	call := kawa.NewCall[users.GetUserByEmailV1Req, users.GetUserByEmailEnvV1Res](c.cfg.Web.HTTPClient, kawa.Post, url).
		WithHeaders(buildAuthHeader(token)).
		WithDeadline(c.deadline()).
		WithRetryPolicy(c.retryPolicy())

	env, err := call.DoWithRetry(ctx, &req)
	if err != nil {
		statusCode, _, _, ok := decodeHTTPError(err)
		if !ok {
			return domain.User{}, err
		}
		if statusCode == 404 {
			return domain.User{}, domain.ErrUserNotFound
		}

		return domain.User{}, translateHTTPError(err)
	}

	if env.Body == nil {
		return domain.User{}, nil
	}

	return domain.User{ID: env.Body.User.ID}, nil
}

// FindUserByEmailAndPassword mirrors
// BaseRepositoryImpl.FindUserByEmailAndPassword. err is
// domain.ErrInvalidCredentials or domain.ErrUserNotFound depending on the
// downstream response's error message.
func (c *Client) FindUserByEmailAndPassword(ctx context.Context, email, password, token string) (domain.User, error) {
	url := fmt.Sprintf("%s/%s", c.cfg.Integration.V1.AuthURL, "users/search/verify-credentials")

	req := users.GetUserByEmailAndPasswordV1Req{Email: email, Password: password}

	call := kawa.NewCall[users.GetUserByEmailAndPasswordV1Req, users.UserV1EnvV1Res](c.cfg.Web.HTTPClient, kawa.Post, url).
		WithHeaders(buildAuthHeader(token)).
		WithDeadline(c.deadline()).
		WithRetryPolicy(c.retryPolicy())

	env, err := call.DoWithRetry(ctx, &req)
	if err != nil {
		_, message, parseErr, ok := decodeHTTPError(err)
		if !ok {
			return domain.User{}, err
		}
		if parseErr != nil {
			return domain.User{}, parseErr
		}

		switch message {
		case "invalid password":
			return domain.User{}, domain.ErrInvalidCredentials
		case "user not found":
			return domain.User{}, domain.ErrUserNotFound
		default:
			return domain.User{}, errors.New(message)
		}
	}

	if env.Body == nil {
		return domain.User{}, nil
	}

	return toDomainUser(env.Body.User), nil
}

// FindUserTokenByEmailAndPassword mirrors
// BaseRepositoryImpl.FindUserTokenByEmailAndPassword: same auth/token
// endpoint as FindTokenByEmailAndPassword, but decoding the refresh_token
// field too. Unlike the source method, this decodes directly into the
// shared (now-fixed) users.GetTokenEnvV1Res rather than a local
// TokenPairEnvV1Res workaround type — see AuthRemote's doc comment.
func (c *Client) FindUserTokenByEmailAndPassword(ctx context.Context, email, password, token string) (string, string, error) {
	url := fmt.Sprintf("%s/%s", c.cfg.Integration.V1.AuthURL, "auth/token")

	req := users.GetTokenV1Req{Email: email, Password: password}

	call := kawa.NewCall[users.GetTokenV1Req, users.GetTokenEnvV1Res](c.cfg.Web.HTTPClient, kawa.Post, url).
		WithHeaders(buildAuthHeader(token)).
		WithDeadline(c.deadline()).
		WithRetryPolicy(c.retryPolicy())

	env, err := call.DoWithRetry(ctx, &req)
	if err != nil {
		return "", "", translateHTTPError(err)
	}

	if env.Body == nil {
		return "", "", nil
	}

	return env.Body.Auth.Token, env.Body.Auth.RefreshToken, nil
}

// RegisterUser mirrors BaseRepositoryImpl.RegisterUser.
func (c *Client) RegisterUser(ctx context.Context, firstName, lastName, email, password, confirmPassword, token, roleID string) (string, error) {
	url := fmt.Sprintf("%s/%s", c.cfg.Integration.V1.AuthURL, "users")

	req := users.RegisterUserV1Req{
		FirstName:       firstName,
		LastName:        lastName,
		Email:           email,
		Password:        password,
		ConfirmPassword: confirmPassword,
		RoleID:          roleID,
	}

	call := kawa.NewCall[users.RegisterUserV1Req, users.RegisterUserEnvV1Res](c.cfg.Web.HTTPClient, kawa.Post, url).
		WithHeaders(buildAuthHeader(token)).
		WithDeadline(c.deadline()).
		WithRetryPolicy(c.retryPolicy())

	env, err := call.DoWithRetry(ctx, &req)
	if err != nil {
		return "", translateHTTPError(err)
	}

	if env.Body == nil {
		return "", nil
	}

	return env.Body.User.ID, nil
}

// SendResetPasswdEmail mirrors BaseRepositoryImpl.SendResetPasswdEmail.
func (c *Client) SendResetPasswdEmail(ctx context.Context, email, token string) error {
	url := fmt.Sprintf("%s/%s", c.cfg.Integration.V1.AuthURL, "users/password/reset")

	req := users.ResetUserPasswordV1Req{Email: email}

	call := kawa.NewCall[users.ResetUserPasswordV1Req, kawa.NoRes](c.cfg.Web.HTTPClient, kawa.Post, url).
		WithHeaders(buildAuthHeader(token)).
		WithDeadline(c.deadline()).
		WithRetryPolicy(c.retryPolicy())

	_, err := call.DoWithRetry(ctx, &req)
	if err != nil {
		return translateHTTPError(err)
	}

	return nil
}

// ActivateUser mirrors BaseRepositoryImpl.ActivateUser. err is
// domain.ErrInvalidActivationToken when the downstream response's error
// message is "token does not match".
func (c *Client) ActivateUser(ctx context.Context, token, userID, activationToken string) error {
	url := fmt.Sprintf("%s/%s/authorize", c.cfg.Integration.V1.AuthURL, "users")

	req := users.ActivateUserV1Req{ID: userID, Token: activationToken}

	call := kawa.NewCall[users.ActivateUserV1Req, kawa.NoRes](c.cfg.Web.HTTPClient, kawa.Patch, url).
		WithHeaders(buildAuthHeader(token)).
		WithDeadline(c.deadline()).
		WithRetryPolicy(c.retryPolicy())

	_, err := call.DoWithRetry(ctx, &req)
	if err != nil {
		_, message, parseErr, ok := decodeHTTPError(err)
		if !ok {
			return err
		}
		if parseErr != nil {
			return parseErr
		}
		if message == "token does not match" {
			return domain.ErrInvalidActivationToken
		}

		return errors.New(message)
	}

	return nil
}

// UpdateUserPassword mirrors BaseRepositoryImpl.UpdateUserPassword.
func (c *Client) UpdateUserPassword(ctx context.Context, userID, password, confirmPassword, token string) error {
	url := fmt.Sprintf("%s/%s/%s/password", c.cfg.Integration.V1.AuthURL, "users", userID)

	req := users.UpdateUserPasswordV1Req{Password: password, ConfirmPassword: confirmPassword}

	call := kawa.NewCall[users.UpdateUserPasswordV1Req, kawa.NoRes](c.cfg.Web.HTTPClient, kawa.Patch, url).
		WithHeaders(buildAuthHeader(token)).
		WithDeadline(c.deadline()).
		WithRetryPolicy(c.retryPolicy())

	_, err := call.DoWithRetry(ctx, &req)
	if err != nil {
		return translateHTTPError(err)
	}

	return nil
}

// HasAddresses mirrors BaseRepositoryImpl.HasAddresses, borrowed from
// address_repository.go in the source repo — see AuthRemote's doc comment
// for why. Note the source method's quirk, preserved here: any downstream
// HTTP error whose body parses as valid JSON is treated as "no addresses,
// no error" — not just the "addresses not found" case its message check
// singles out (the original's reassignment of err to the (nil-on-success)
// json.Unmarshal result discards the actual failure reason). Only a body
// that fails to parse as JSON, or a non-HTTP error (network failure),
// propagates as an error here.
func (c *Client) HasAddresses(ctx context.Context, userID, token string) (bool, string, error) {
	url := fmt.Sprintf("%s/%s/%s/%s", c.cfg.Integration.V1.AuthURL, "users", userID, "addresses")

	call := kawa.NewCall[kawa.NoReq, users.GetAddressesV1EnvV1Res](c.cfg.Web.HTTPClient, kawa.Get, url).
		WithHeaders(buildAuthHeader(token)).
		WithDeadline(c.deadline()).
		WithRetryPolicy(c.retryPolicy())

	env, err := call.DoWithRetry(ctx, nil)
	if err != nil {
		var errHTTP kawa.ErrInvalidHTTPStatus
		if errors.As(err, &errHTTP) {
			var em errMessage
			if unmarshalErr := json.Unmarshal(errHTTP.Body, &em); unmarshalErr != nil {
				return false, "", unmarshalErr
			}

			return false, "", nil
		}

		return false, "", err
	}

	if env.Body == nil || len(env.Body.Addresses) == 0 {
		return false, "", nil
	}

	return true, env.Body.Addresses[0].ID, nil
}

// refreshTokenV1Req mirrors the local RefreshTokenV1Req type in the
// source repo's user_repository.go: beecore-customers' /auth/refresh
// request shape — {user_id, refresh_token}.
type refreshTokenV1Req struct {
	UserID       string `json:"user_id,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

func (r refreshTokenV1Req) Req() {}

// RefreshUserToken mirrors BaseRepositoryImpl.RefreshUserToken: POST
// auth/refresh, exchanging a refresh token for a new access token without
// the user's password. Decodes into the shared (now-fixed)
// users.GetTokenEnvV1Res — see AuthRemote's doc comment — but only the
// access token is used here (the caller doesn't rotate its stored refresh
// token; /auth/refresh doesn't rotate it either).
//
// err is domain.ErrSessionRejected when the downstream response is
// 401/403 — mirrors isSessionRejected in the source repo's
// cmd/web/helpers.go.
func (c *Client) RefreshUserToken(ctx context.Context, userID, refreshToken, token string) (string, error) {
	url := fmt.Sprintf("%s/%s", c.cfg.Integration.V1.AuthURL, "auth/refresh")

	req := refreshTokenV1Req{UserID: userID, RefreshToken: refreshToken}

	call := kawa.NewCall[refreshTokenV1Req, users.GetTokenEnvV1Res](c.cfg.Web.HTTPClient, kawa.Post, url).
		WithHeaders(buildAuthHeader(token)).
		WithDeadline(c.deadline()).
		WithRetryPolicy(c.retryPolicy())

	env, err := call.DoWithRetry(ctx, &req)
	if err != nil {
		statusCode, _, _, ok := decodeHTTPError(err)
		if ok && (statusCode == 401 || statusCode == 403) {
			return "", domain.ErrSessionRejected
		}

		return "", translateHTTPError(err)
	}

	if env.Body == nil {
		return "", nil
	}

	return env.Body.Auth.Token, nil
}
