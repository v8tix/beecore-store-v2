// Package address is the address use-case, ported from
// internal/business/core/repository/address_repository.go in the source
// repo at the service layer (see port/service.Address's doc comment for
// the full mapping and its callers). It's mostly a thin pass-through to
// resource.AddressRemote — address_repository.go's methods have no
// orchestration logic of their own in the source repo — resolving its own
// admin token via resource.AuthRemote first, the same pattern
// core/user.Service uses.
//
// Insert/Update also apply the Ecuador country-code phone prefix here
// rather than in the handler: the source repo's newAddress/updateAddress/
// createAddressWizard handlers each compute
// fmt.Sprintf("+593%s", form.Phone) themselves before calling the
// repository, but core/user.Service already relocated that exact
// computation for UpdateUser into the service layer (see its
// phoneCountryCode const) — applying the same relocation here keeps every
// address-writing handler from needing its own copy of the prefix logic.
package address

import (
	"context"
	"fmt"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/core/port/resource"
	"github.com/v8tix/beecore-store-v2/internal/core/port/service"
)

// phoneCountryCode is the Ecuador country-code prefix the source repo's
// address handlers hardcode onto every phone number before sending it
// downstream — see the package doc comment.
const phoneCountryCode = "+593"

// Dependencies are addressService's collaborators, all resolved once at
// composition-root time rather than read from *config.Cfg here — core
// stays free of any infra/config import.
type Dependencies struct {
	AddressRemote resource.AddressRemote

	// AuthRemote is used only for its GetAdminToken method — reused from
	// the auth slice's resource.AuthRemote rather than duplicated here.
	AuthRemote resource.AuthRemote
}

type addressService struct {
	deps Dependencies
}

var _ service.Address = (*addressService)(nil)

func NewAddressService(d Dependencies) *addressService {
	return &addressService{deps: d}
}

// FindByUserID mirrors Repository.FindAddressesByUserID exposed at the
// service layer.
func (s *addressService) FindByUserID(ctx context.Context, userID string) ([]domain.Address, error) {
	at, err := s.deps.AuthRemote.GetAdminToken(ctx)
	if err != nil {
		return nil, err
	}

	return s.deps.AddressRemote.FindAddressesByUserID(ctx, userID, at)
}

// HasAddresses mirrors Repository.HasAddresses exposed at the service
// layer.
func (s *addressService) HasAddresses(ctx context.Context, userID string) (bool, string, error) {
	at, err := s.deps.AuthRemote.GetAdminToken(ctx)
	if err != nil {
		return false, "", err
	}

	return s.deps.AddressRemote.HasAddresses(ctx, userID, at)
}

// Insert mirrors Repository.InsertAddress exposed at the service layer,
// prefixing phone with the Ecuador country code (see the package doc
// comment).
func (s *addressService) Insert(
	ctx context.Context,
	userID string,
	addressType domain.AddressType,
	country, city, state, zip, mainStreet, secondaryStreet, numeration, phone string,
) (string, error) {
	at, err := s.deps.AuthRemote.GetAdminToken(ctx)
	if err != nil {
		return "", err
	}

	return s.deps.AddressRemote.InsertAddress(ctx, userID, addressType, country, city, state, zip, mainStreet, secondaryStreet, numeration, fmt.Sprintf("%s%s", phoneCountryCode, phone), at)
}

// Update mirrors Repository.UpdateAddress exposed at the service layer,
// prefixing phone with the Ecuador country code (see the package doc
// comment).
func (s *addressService) Update(
	ctx context.Context,
	id, userID string,
	addressType domain.AddressType,
	country, city, state, zip, mainStreet, secondaryStreet, numeration, phone string,
) error {
	at, err := s.deps.AuthRemote.GetAdminToken(ctx)
	if err != nil {
		return err
	}

	return s.deps.AddressRemote.UpdateAddress(ctx, id, userID, addressType, country, city, state, zip, mainStreet, secondaryStreet, numeration, fmt.Sprintf("%s%s", phoneCountryCode, phone), at)
}
