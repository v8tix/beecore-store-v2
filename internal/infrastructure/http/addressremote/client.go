// Package addressremote is the kawa-based HTTP client implementing
// resource.AddressRemote, ported 1:1 from
// internal/business/core/repository/address_repository.go in the source
// repo (see port/resource.AddressRemote's doc comment for the full
// mapping, including HasAddresses's relocation from
// infrastructure/http/authremote).
package addressremote

import (
	"fmt"
	"time"

	"github.com/v8tix/beecore-eda/config"
	users "github.com/v8tix/beecore-http/messages/users/v1"
	"github.com/v8tix/kawa"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/core/port/resource"
	"github.com/v8tix/beecore-store-v2/internal/infrastructure/http/httpshared"
)

// Client is the outbound HTTP boundary for the address vertical slice. It
// holds *config.Cfg directly (same as source repo's BaseRepositoryImpl
// struct) — the URLs and shared *http.Client it needs all live there
// already.
type Client struct {
	cfg *config.Cfg
}

var _ resource.AddressRemote = (*Client)(nil)

func NewClient(cfg *config.Cfg) *Client {
	return &Client{cfg: cfg}
}

// deadline and retryPolicy mirror BaseRepositoryImpl.deadline/retryPolicy
// in the source repo: every call here is a single-record lookup, so they
// use the config-tunable "fast" HTTP client profile.
func (c *Client) deadline() time.Duration {
	return httpshared.Deadline(c.cfg)
}

func (c *Client) retryPolicy() kawa.RetryPolicy {
	return httpshared.RetryPolicy(c.cfg)
}

// translateHTTPError turns a kawa.ErrInvalidHTTPStatus into a plain error
// carrying the downstream response body's parsed message, for call sites
// that don't need to distinguish specific downstream rejection reasons.
// Non-HTTP errors (network failures) pass through unchanged.
//
// TODO(httpshared): this diverges from authremote's translateHTTPError —
// see that package's TODO — unifying the two is deferred to its own
// follow-up commit rather than folded into this dedup pass.
func translateHTTPError(err error) error {
	statusCode, message, parseErr, ok := httpshared.DecodeHTTPError(err)
	if !ok {
		return err
	}
	if parseErr != nil {
		return parseErr
	}

	return fmt.Errorf("downstream request failed with status %d: %s", statusCode, message)
}

// checkAddressType mirrors repository.CheckAddressType in the source
// repo: addressType must be one of the domain.AddressType constants
// before any downstream call is made.
func checkAddressType(addressType domain.AddressType) error {
	switch addressType {
	case domain.PersonalAddress, domain.StoreAddress, domain.ShippingAddress:
		return nil
	default:
		return domain.ErrBadAddressType
	}
}

func toDomainAddress(a users.Address) domain.Address {
	return domain.Address{
		ID:              a.ID,
		UserID:          a.UserID,
		Type:            domain.AddressType(a.Type),
		Country:         a.Country,
		State:           a.State,
		City:            a.City,
		PostalCode:      a.PostalCode,
		MainStreet:      a.MainStreet,
		SecondaryStreet: a.SecondaryStreet,
		Numeration:      a.Numeration,
		Phone:           a.Phone,
		CreatedAt:       a.CreatedAt,
		UpdatedAt:       a.UpdatedAt,
	}
}
