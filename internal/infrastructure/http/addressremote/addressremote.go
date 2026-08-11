package addressremote

import (
	"context"
	"fmt"

	users "github.com/v8tix/beecore-http/messages/users/v1"
	"github.com/v8tix/kawa"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/infrastructure/http/httpshared"
)

// FindAddressesByUserID mirrors BaseRepositoryImpl.FindAddressesByUserID.
func (c *Client) FindAddressesByUserID(ctx context.Context, userID, token string) ([]domain.Address, error) {
	url := fmt.Sprintf("%s/%s/%s/%s", c.cfg.Integration.V1.AuthURL, "users", userID, "addresses")

	call := kawa.NewCall[kawa.NoReq, users.GetAddressesV1EnvV1Res](c.cfg.Web.HTTPClient, kawa.Get, url).
		WithHeaders(httpshared.BuildAuthHeader(token)).
		WithDeadline(c.deadline()).
		WithRetryPolicy(c.retryPolicy())

	env, err := call.DoWithRetry(ctx, nil)
	if err != nil {
		return nil, translateHTTPError(err)
	}

	if env.Body == nil {
		return nil, nil
	}

	addresses := make([]domain.Address, 0, len(env.Body.Addresses))
	for _, a := range env.Body.Addresses {
		addresses = append(addresses, toDomainAddress(a))
	}

	return addresses, nil
}

// HasAddresses mirrors BaseRepositoryImpl.HasAddresses, relocated here
// from infrastructure/http/authremote.Client — see AddressRemote's doc
// comment for why. Note the source method's quirk, preserved here: any
// downstream HTTP error whose body parses as valid JSON is treated as "no
// addresses, no error" — not just the "addresses not found" case its
// message check singles out (the original's reassignment of err to the
// (nil-on-success) json.Unmarshal result discards the actual failure
// reason). Only a body that fails to parse as JSON, or a non-HTTP error
// (network failure), propagates as an error here.
//
// This is a self-contained implementation (not built on top of
// FindAddressesByUserID above) to preserve that exact swallowing behavior
// unchanged from its prior home on AuthRemote — FindAddressesByUserID
// instead translates every downstream error generically, which would lose
// the distinction this method's callers rely on.
func (c *Client) HasAddresses(ctx context.Context, userID, token string) (bool, string, error) {
	url := fmt.Sprintf("%s/%s/%s/%s", c.cfg.Integration.V1.AuthURL, "users", userID, "addresses")

	call := kawa.NewCall[kawa.NoReq, users.GetAddressesV1EnvV1Res](c.cfg.Web.HTTPClient, kawa.Get, url).
		WithHeaders(httpshared.BuildAuthHeader(token)).
		WithDeadline(c.deadline()).
		WithRetryPolicy(c.retryPolicy())

	env, err := call.DoWithRetry(ctx, nil)
	if err != nil {
		_, _, parseErr, ok := httpshared.DecodeHTTPError(err)
		if !ok {
			return false, "", err
		}
		if parseErr != nil {
			return false, "", parseErr
		}

		return false, "", nil
	}

	if env.Body == nil || len(env.Body.Addresses) == 0 {
		return false, "", nil
	}

	return true, env.Body.Addresses[0].ID, nil
}

// InsertAddress mirrors BaseRepositoryImpl.InsertAddress. err is
// domain.ErrBadAddressType when addressType isn't one of the
// domain.AddressType constants (checked locally, mirroring
// repository.CheckAddressType, before any downstream call is made);
// domain.ErrMaxAddressesReached when the downstream service's error
// message is "maximum address length reached"; or a generic error on any
// other downstream rejection.
func (c *Client) InsertAddress(
	ctx context.Context,
	userID string,
	addressType domain.AddressType,
	country, city, state, zip, mainStreet, secondaryStreet, numeration, phone, token string,
) (string, error) {
	if err := checkAddressType(addressType); err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s/%s/%s/%s", c.cfg.Integration.V1.AuthURL, "users", userID, "addresses")

	req := users.RegisterAddressV1Req{
		Type:            string(addressType),
		Country:         country,
		State:           state,
		City:            city,
		PostalCode:      zip,
		MainStreet:      mainStreet,
		SecondaryStreet: secondaryStreet,
		Numeration:      numeration,
		Phone:           phone,
	}

	call := kawa.NewCall[users.RegisterAddressV1Req, users.RegisterAddressEnvV1Res](c.cfg.Web.HTTPClient, kawa.Post, url).
		WithHeaders(httpshared.BuildAuthHeader(token)).
		WithDeadline(c.deadline()).
		WithRetryPolicy(c.retryPolicy())

	env, err := call.DoWithRetry(ctx, &req)
	if err != nil {
		_, message, parseErr, ok := httpshared.DecodeHTTPError(err)
		if !ok {
			return "", err
		}
		if parseErr != nil {
			return "", parseErr
		}
		if message == "maximum address length reached" {
			return "", domain.ErrMaxAddressesReached
		}

		return "", translateHTTPError(err)
	}

	if env.Body == nil {
		return "", nil
	}

	return env.Body.Address.ID, nil
}

// UpdateAddress mirrors BaseRepositoryImpl.UpdateAddress. err is
// domain.ErrBadAddressType when addressType isn't one of the
// domain.AddressType constants (checked locally, before any downstream
// call is made), or a generic error on any other downstream rejection.
func (c *Client) UpdateAddress(
	ctx context.Context,
	id, userID string,
	addressType domain.AddressType,
	country, city, state, zip, mainStreet, secondaryStreet, numeration, phone, token string,
) error {
	if err := checkAddressType(addressType); err != nil {
		return err
	}

	url := fmt.Sprintf("%s/%s/%s/%s/%s", c.cfg.Integration.V1.AuthURL, "users", userID, "addresses", id)

	req := users.UpdateAddressV1Req{
		Type:            string(addressType),
		Country:         country,
		State:           state,
		City:            city,
		PostalCode:      zip,
		MainStreet:      mainStreet,
		SecondaryStreet: secondaryStreet,
		Numeration:      numeration,
		Phone:           phone,
	}

	call := kawa.NewCall[users.UpdateAddressV1Req, kawa.NoRes](c.cfg.Web.HTTPClient, kawa.Put, url).
		WithHeaders(httpshared.BuildAuthHeader(token)).
		WithDeadline(c.deadline()).
		WithRetryPolicy(c.retryPolicy())

	_, err := call.DoWithRetry(ctx, &req)
	if err != nil {
		return translateHTTPError(err)
	}

	return nil
}
