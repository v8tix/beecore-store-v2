package service

import (
	"context"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
)

//go:generate mockery --name Address

// Address is the address use-case, ported from
// internal/business/core/repository/address_repository.go in the source
// repo at the service layer, for the standalone address-management flows
// that call it directly: the source repo's cmd/web/address_handler.go
// (newAddress/updateAddress, "/addresses/new" and "/addresses/{address_id}")
// and the create-address wizard step of cmd/web/handlers.go
// (createAddressWizard, "/user/create/address" — grouped with address
// concerns here despite its URL prefix; see AddressHandler's doc comment).
//
// Unlike resource.AddressRemote, every method here drops the trailing
// token parameter: this service resolves its own admin token via
// AuthRemote.GetAdminToken, the same pattern core/user.Service.UpdateProfile
// already uses, since every handler that calls this service reaches it
// directly rather than through another use-case that already has a token
// in hand. core/auth.Login is the one caller in the opposite position — it
// already holds its own admin token from its own GetAdminToken call — so
// it depends on resource.AddressRemote directly instead of this service,
// same "services don't depend on other services" rule applied in
// beecore-admin-v2.
type Address interface {
	// FindByUserID lists userID's addresses. Mirrors
	// Repository.FindAddressesByUserID at the service layer.
	FindByUserID(ctx context.Context, userID string) ([]domain.Address, error)

	// HasAddresses reports whether userID has at least one registered
	// address, and its first address's ID if so. Mirrors
	// Repository.HasAddresses at the service layer. No handler in this
	// repo calls this method today (core/auth.Login depends on
	// resource.AddressRemote directly instead, per this interface's doc
	// comment) — kept for parity with address_repository.go's full method
	// set, same precedent as beecore-admin-v2's port/service.Address.
	HasAddresses(ctx context.Context, userID string) (hasAddresses bool, firstAddressID string, err error)

	// Insert registers a new address for userID, of the given
	// addressType. err is domain.ErrBadAddressType or
	// domain.ErrMaxAddressesReached on the matching downstream rejection,
	// or a generic error otherwise. Mirrors Repository.InsertAddress at
	// the service layer.
	Insert(ctx context.Context, userID string, addressType domain.AddressType, country, city, state, zip, mainStreet, secondaryStreet, numeration, phone string) (addressID string, err error)

	// Update updates an existing address (id) belonging to userID. err is
	// domain.ErrBadAddressType on a malformed address type, or a generic
	// error otherwise. Mirrors Repository.UpdateAddress at the service
	// layer.
	Update(ctx context.Context, id, userID string, addressType domain.AddressType, country, city, state, zip, mainStreet, secondaryStreet, numeration, phone string) error
}
