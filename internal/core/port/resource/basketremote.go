package resource

import (
	"context"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
)

//go:generate mockery --name BasketRemote

// BasketRemote is the outbound HTTP boundary for the basket slice, ported
// from internal/business/core/repository/baskets_repository.go in the
// source repo. Every method name below matches its source counterpart;
// signatures gain a leading ctx and drop the kawa.Envelope/*_V1Req/*_V1Res
// wire shape in favor of domain types or primitives — translating a
// downstream response into a plain Go error (or domain.ErrBasketCheckedOut
// on a 409 Conflict, see BasketAddItem below) is this adapter's job, so
// core/basket and its handlers never need to know kawa exists.
type BasketRemote interface {
	// CreateBasket starts a new basket for userID. Mirrors
	// BaseRepositoryImpl.CreateBasket (POST baskets). Called both by
	// core/user's wizard flow (the source repo's updateUserWizard handler's
	// CreateBasket step) and, via BasketAddItem's conflict-retry path, by
	// core/basket itself.
	CreateBasket(ctx context.Context, userID, token string) (basketID string, err error)

	// FindBasket looks up a basket's full contents by id. Mirrors
	// BaseRepositoryImpl.FindBasket (GET baskets/{id}). Note the downstream
	// response (baskets.GetBasketV1Res) doesn't carry the basket's own ID —
	// see domain.Basket's doc comment — so the adapter copies the input id
	// onto the returned domain.Basket directly.
	FindBasket(ctx context.Context, id, token string) (domain.Basket, error)

	// BasketAddItem adds quantity of productID to basket id. err is
	// domain.ErrBasketCheckedOut on a downstream 409 Conflict — the source
	// repo's cmd/web/products_handler.go searchAddItem handler treats that
	// as "the session's basket may already be checked out" and
	// transparently starts a fresh one (see CreateBasket above). Mirrors
	// BaseRepositoryImpl.BasketAddItem (PUT baskets/{id}/addItem).
	BasketAddItem(ctx context.Context, id, productID string, quantity int, token string) error

	// BasketRemoveItem removes quantity of productID from basket id.
	// Mirrors BaseRepositoryImpl.BasketRemoveItem (PUT
	// baskets/{id}/removeItem).
	BasketRemoveItem(ctx context.Context, id, productID string, quantity int, token string) error

	// CheckoutBasket marks basket id as checked out against paymentID and
	// shippingAddressID. Mirrors BaseRepositoryImpl.CheckoutBasket (PUT
	// baskets/{id}/checkout). Not yet called from core/basket's use-case —
	// the source repo's own caller (cmd/web/payphone_handlers.go) is a
	// Payment-slice concern (plan Task 17/18) — ported alongside every
	// other baskets_repository.go method for full parity with
	// BaseRepository's method set, same precedent as
	// resource.AuthRemote's RefreshUserToken.
	CheckoutBasket(ctx context.Context, id, paymentID, shippingAddressID, token string) error

	// CancelBasket deletes basket id outright. Mirrors
	// BaseRepositoryImpl.CancelBasket (DELETE baskets/{id}). Unused by any
	// live source handler today — its only source call site is
	// commented-out dead PayPal code (cmd/web/paypal_handlers.go) — ported
	// for the same full-parity reason as CheckoutBasket above.
	CancelBasket(ctx context.Context, id, token string) error

	// FindBasketByUserID looks up the ID of userID's currently open
	// basket. Mirrors BaseRepositoryImpl.FindBasketByUserID (GET
	// baskets/opened/users/{id}) — note the downstream response
	// (baskets.GetBasketByUserIDEnvV1Res) only ever carries a basket ID,
	// not a full basket, hence the narrower return type here vs.
	// FindBasket's. Used by core/auth.Login's basket-quantity-cookie step
	// (AuthHandler.Login).
	FindBasketByUserID(ctx context.Context, userID, token string) (basketID string, err error)

	// ComputeFinancials returns basket id's cost breakdown. Mirrors
	// BaseRepositoryImpl.ComputeFinancials (GET baskets/{id}/financials).
	ComputeFinancials(ctx context.Context, id, token string) (domain.Financials, error)
}
