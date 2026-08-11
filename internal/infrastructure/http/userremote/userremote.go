package userremote

import (
	"context"
	"fmt"

	users "github.com/v8tix/beecore-http/messages/users/v1"
	"github.com/v8tix/kawa"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/infrastructure/http/httpshared"
)

// UpdateUser mirrors BaseRepositoryImpl.UpdateUser. err is
// domain.ErrUserNotFound on a downstream 404 with a JSON-parseable error
// body — the source updateUserWizard handler's own special-case for this
// exact call, kept here as a domain sentinel so the use-case/handler
// above don't need to know kawa exists. Any other downstream error is
// translated to a plain wrapped error carrying the downstream response
// body; a body that fails to parse as errMessage JSON propagates as that
// parse error.
func (c *Client) UpdateUser(
	ctx context.Context,
	id, dni, firstName, lastName, birthday, phone, imageURL, website, token string,
) error {
	url := fmt.Sprintf("%s/%s/%s", c.cfg.Integration.V1.AuthURL, "users", id)

	req := users.UpdateUserV1Req{
		DNI:       dni,
		FirstName: firstName,
		LastName:  lastName,
		Birthday:  birthday,
		Genre:     "",
		Phone:     phone,
		ImgURL:    imageURL,
		Website:   website,
	}

	call := kawa.NewCall[users.UpdateUserV1Req, kawa.NoRes](c.cfg.Web.HTTPClient, kawa.Put, url).
		WithHeaders(httpshared.BuildAuthHeader(token)).
		WithDeadline(c.deadline()).
		WithRetryPolicy(c.retryPolicy())

	_, err := call.DoWithRetry(ctx, &req)
	if err != nil {
		statusCode, message, parseErr, ok := httpshared.DecodeHTTPError(err)
		if !ok {
			return err
		}

		if statusCode == 404 {
			return domain.ErrUserNotFound
		}

		if parseErr != nil {
			return parseErr
		}

		return fmt.Errorf("downstream request failed with status %d: %s", statusCode, message)
	}

	return nil
}
