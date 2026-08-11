package service

import (
	"context"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
)

//go:generate mockery --name Basket

// Basket is the basket use-case, ported from
// internal/business/core/repository/baskets_repository.go in the source
// repo at the service layer, for every handler that touches a basket: the
// source repo's cmd/web/basket_handler.go (basket/basketAddItem/
// basketRemoveItem, "/basket" and "/basket/{add,remove}-item/{product_id}"),
// its searchAddItem handler (cmd/web/products_handler.go, "/search/add" —
// grouped with basket concerns here despite its source file, see
// ProductHandler's doc comment), and the basket-dependent steps of
// AuthHandler.Login and UserHandler.UpdateWizard that core/auth.Login and
// core/user's wizard flow deferred until this slice existed (see both
// package doc comments).
//
// Like core/address.Service and core/product.Service, every method here
// drops the trailing token parameter: this service resolves its own admin
// token via AuthRemote.GetAdminToken. resource.BasketRemote's
// CheckoutBasket/CancelBasket are deliberately not mirrored here — no
// handler in this repo calls them yet (CheckoutBasket's only source
// caller, cmd/web/payphone_handlers.go, is a Payment-slice concern, plan
// Task 17/18); when that slice lands, its service depends on
// resource.BasketRemote directly for them, the same "services don't
// depend on other services" rule core/auth.Login already applies to
// resource.AddressRemote.
type Basket interface {
	// ComputeFinancials returns basketID's cost breakdown. Mirrors
	// Repository.ComputeFinancials at the service layer — used by the
	// source repo's basket handler to render the shopping-cart page's
	// financial line items and grand total.
	ComputeFinancials(ctx context.Context, basketID string) (domain.Financials, error)

	// AddItem adds one unit of productID to basketID, then returns the
	// basket's up-to-date contents. Mirrors the source repo's
	// basketAddItem handler's BasketAddItem+FindBasket call sequence at
	// the service layer — the caller uses the returned domain.Basket's
	// Items to recompute the browser cookie's item-quantity display
	// (session.Values[keys.SessionBasketItems] in the source repo).
	AddItem(ctx context.Context, basketID, productID string) (domain.Basket, error)

	// RemoveItem removes one unit of productID from basketID, then
	// returns the basket's up-to-date contents. Mirrors the source repo's
	// basketRemoveItem handler's BasketRemoveItem+FindBasket call sequence
	// at the service layer, same as AddItem.
	RemoveItem(ctx context.Context, basketID, productID string) (domain.Basket, error)

	// AddItemFromSearch mirrors the source repo's searchAddItem handler:
	// adds productID to basketID, transparently starting (and returning)
	// a fresh basket for userID if the existing one is already checked
	// out (domain.ErrBasketCheckedOut on a downstream 409 — "a stale tab
	// from a completed order"), then returns the (possibly new) basket
	// ID together with the basket's up-to-date contents either way, so
	// the caller can update both the cookie's basket-ID value and its
	// item-quantity display.
	AddItemFromSearch(ctx context.Context, basketID, productID, userID string) (newBasketID string, basket domain.Basket, err error)

	// FindByUserID looks up the ID of userID's currently open basket.
	// Mirrors Repository.FindBasketByUserID at the service layer. Used by
	// AuthHandler.Login's basket-quantity-cookie step, resolving
	// core/auth.Login's own deferral of that step (see its doc comment).
	FindByUserID(ctx context.Context, userID string) (basketID string, err error)

	// Create starts a new basket for userID. Mirrors Repository.CreateBasket
	// at the service layer. Used by UserHandler.UpdateWizard's deferred
	// CreateBasket sub-step (see its doc comment).
	Create(ctx context.Context, userID string) (basketID string, err error)

	// FindByID looks up a basket's full contents by id. Mirrors
	// Repository.FindBasket at the service layer. Used by AuthHandler.Login's
	// basket-quantity-cookie step to compute the initial item count once a
	// fully onboarded user is redirected straight to /search.
	FindByID(ctx context.Context, id string) (domain.Basket, error)
}
