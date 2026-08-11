// Package basket is the basket use-case, ported from
// internal/business/core/repository/baskets_repository.go in the source
// repo at the service layer (see port/service.Basket's doc comment for the
// full mapping and its callers). AddItem/RemoveItem/AddItemFromSearch
// carry real orchestration (a mutation followed by a fresh read, and — for
// AddItemFromSearch — a conflict-triggered basket recreation and retry),
// mirroring the multi-call sequences their source handler counterparts
// perform directly against BaseRepositoryImpl; every other method is a
// thin pass-through to resource.BasketRemote, resolving its own admin
// token via resource.AuthRemote first, the same pattern core/address.Service
// and core/product.Service use.
package basket

import (
	"context"
	"errors"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/core/port/resource"
	"github.com/v8tix/beecore-store-v2/internal/core/port/service"
)

// Dependencies are basketService's collaborators, all resolved once at
// composition-root time rather than read from *config.Cfg here — core
// stays free of any infra/config import.
type Dependencies struct {
	BasketRemote resource.BasketRemote

	// AuthRemote is used only for its GetAdminToken method — reused from
	// the auth slice's resource.AuthRemote rather than duplicated here.
	AuthRemote resource.AuthRemote
}

type basketService struct {
	deps Dependencies
}

var _ service.Basket = (*basketService)(nil)

func NewBasketService(d Dependencies) *basketService {
	return &basketService{deps: d}
}

// ComputeFinancials mirrors Repository.ComputeFinancials exposed at the
// service layer.
func (s *basketService) ComputeFinancials(ctx context.Context, basketID string) (domain.Financials, error) {
	at, err := s.deps.AuthRemote.GetAdminToken(ctx)
	if err != nil {
		return domain.Financials{}, err
	}

	return s.deps.BasketRemote.ComputeFinancials(ctx, basketID, at)
}

// AddItem mirrors the source repo's basketAddItem handler's
// BasketAddItem+FindBasket call sequence, exposed at the service layer.
func (s *basketService) AddItem(ctx context.Context, basketID, productID string) (domain.Basket, error) {
	at, err := s.deps.AuthRemote.GetAdminToken(ctx)
	if err != nil {
		return domain.Basket{}, err
	}

	if err := s.deps.BasketRemote.BasketAddItem(ctx, basketID, productID, 1, at); err != nil {
		return domain.Basket{}, err
	}

	return s.deps.BasketRemote.FindBasket(ctx, basketID, at)
}

// RemoveItem mirrors the source repo's basketRemoveItem handler's
// BasketRemoveItem+FindBasket call sequence, exposed at the service layer.
func (s *basketService) RemoveItem(ctx context.Context, basketID, productID string) (domain.Basket, error) {
	at, err := s.deps.AuthRemote.GetAdminToken(ctx)
	if err != nil {
		return domain.Basket{}, err
	}

	if err := s.deps.BasketRemote.BasketRemoveItem(ctx, basketID, productID, 1, at); err != nil {
		return domain.Basket{}, err
	}

	return s.deps.BasketRemote.FindBasket(ctx, basketID, at)
}

// AddItemFromSearch mirrors the source repo's searchAddItem handler: add
// productID to basketID; on domain.ErrBasketCheckedOut, start a fresh
// basket for userID and retry once; either way, return the (possibly new)
// basket ID alongside its up-to-date contents.
func (s *basketService) AddItemFromSearch(ctx context.Context, basketID, productID, userID string) (string, domain.Basket, error) {
	at, err := s.deps.AuthRemote.GetAdminToken(ctx)
	if err != nil {
		return "", domain.Basket{}, err
	}

	err = s.deps.BasketRemote.BasketAddItem(ctx, basketID, productID, 1, at)
	if errors.Is(err, domain.ErrBasketCheckedOut) {
		newBasketID, createErr := s.deps.BasketRemote.CreateBasket(ctx, userID, at)
		if createErr != nil {
			return "", domain.Basket{}, createErr
		}

		basketID = newBasketID
		err = s.deps.BasketRemote.BasketAddItem(ctx, basketID, productID, 1, at)
	}
	if err != nil {
		return "", domain.Basket{}, err
	}

	basket, err := s.deps.BasketRemote.FindBasket(ctx, basketID, at)
	if err != nil {
		return "", domain.Basket{}, err
	}

	return basketID, basket, nil
}

// FindByUserID mirrors Repository.FindBasketByUserID exposed at the
// service layer.
func (s *basketService) FindByUserID(ctx context.Context, userID string) (string, error) {
	at, err := s.deps.AuthRemote.GetAdminToken(ctx)
	if err != nil {
		return "", err
	}

	return s.deps.BasketRemote.FindBasketByUserID(ctx, userID, at)
}

// Create mirrors Repository.CreateBasket exposed at the service layer.
func (s *basketService) Create(ctx context.Context, userID string) (string, error) {
	at, err := s.deps.AuthRemote.GetAdminToken(ctx)
	if err != nil {
		return "", err
	}

	return s.deps.BasketRemote.CreateBasket(ctx, userID, at)
}

// FindByID mirrors Repository.FindBasket exposed at the service layer.
func (s *basketService) FindByID(ctx context.Context, id string) (domain.Basket, error) {
	at, err := s.deps.AuthRemote.GetAdminToken(ctx)
	if err != nil {
		return domain.Basket{}, err
	}

	return s.deps.BasketRemote.FindBasket(ctx, id, at)
}
