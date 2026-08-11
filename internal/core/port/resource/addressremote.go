package resource

import (
	"context"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
)

//go:generate mockery --name AddressRemote

// AddressRemote is the outbound HTTP boundary for the address slice,
// ported from internal/business/core/repository/address_repository.go in
// the source repo. Every method name below matches its source
// counterpart; signatures gain a leading ctx and drop the
// kawa.Envelope/*_V1Req/*_V1Res wire shape in favor of domain types or
// primitives — translating a downstream response into a plain Go error
// (or one of the domain.Err* sentinels) is this adapter's job, so
// core/address, core/auth and their handlers never need to know kawa
// exists.
//
// HasAddresses used to live on resource.AuthRemote — a documented
// placeholder borrowed there because the login use-case (core/auth) needs
// it to populate domain.Session's UserShippingAddress field, and this
// Address vertical slice didn't exist yet (plan Task 8). Now that it
// does, HasAddresses lives here, its true owner, and core/auth.Dependencies
// holds an AddressRemote field instead of relying on AuthRemote for this —
// this was a single call site, low risk to move, same discipline used for
// any other borrowed-method relocation.
type AddressRemote interface {
	// FindAddressesByUserID lists every address registered to userID.
	// Mirrors Repository.FindAddressesByUserID. Used both by HasAddresses
	// below and directly by the standalone address-management handler to
	// look up an existing address's fields before editing (the source
	// repo's cmd/web/address_handler.go updateAddress handler).
	FindAddressesByUserID(ctx context.Context, userID, token string) ([]domain.Address, error)

	// HasAddresses reports whether userID has at least one registered
	// address, and its first address's ID if so. Mirrors
	// Repository.HasAddresses, including its quirk (preserved 1:1 from
	// its prior home on AuthRemote): any downstream HTTP error whose body
	// parses as valid JSON is treated as "no addresses, no error" — not
	// just the "addresses not found" case the source's message check
	// singles out. Only a body that fails to parse as JSON, or a
	// non-HTTP error (network failure), propagates as an error here.
	HasAddresses(ctx context.Context, userID, token string) (hasAddresses bool, firstAddressID string, err error)

	// InsertAddress registers a new address for userID, of the given
	// addressType. err is domain.ErrBadAddressType when addressType isn't
	// one of the domain.AddressType constants (checked locally, mirroring
	// repository.CheckAddressType, before any downstream call is made);
	// domain.ErrMaxAddressesReached when the downstream service rejects
	// the insert because userID already has the maximum number of
	// addresses allowed (the source repo's cmd/web/address_handler.go
	// newAddress handler special-cases this exact message); or a generic
	// error on any other downstream rejection. Mirrors
	// Repository.InsertAddress.
	InsertAddress(ctx context.Context, userID string, addressType domain.AddressType, country, city, state, zip, mainStreet, secondaryStreet, numeration, phone, token string) (addressID string, err error)

	// UpdateAddress updates an existing address (id) belonging to userID.
	// err is domain.ErrBadAddressType when addressType isn't one of the
	// domain.AddressType constants, or a generic error on any other
	// downstream rejection. Mirrors Repository.UpdateAddress.
	UpdateAddress(ctx context.Context, id, userID string, addressType domain.AddressType, country, city, state, zip, mainStreet, secondaryStreet, numeration, phone, token string) error
}
