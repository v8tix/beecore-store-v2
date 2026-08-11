package service

import (
	"context"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
)

//go:generate mockery --name Order

// Order is the order use-case, ported from
// internal/business/core/repository/orders_repository.go in the source
// repo at the service layer, for the source repo's cmd/web/order_handler.go
// orders handler ("/orders", the order-history page).
//
// Like core/basket.Service, this service resolves its own admin token via
// AuthRemote.GetAdminToken rather than taking one as a parameter.
//
// GetByUserID is the only method here: it is the only order operation any
// handler built so far in this repo actually calls (confirmOrder/
// cancelOrder in the source repo are static template renders with no
// repository call at all — see OrderHandler's doc comment). CreateOrder/
// GetOrder/CancelOrder/CompleteOrder/ReadyOrder are NOT mirrored here —
// resource.OrderRemote carries the full method set for full parity (see
// its doc comment), but no handler in this repo calls them yet
// (CreateOrder's only source caller, cmd/web/payphone_handlers.go, is a
// Payment-slice concern, plan Task 17/18); when that slice lands, its own
// service depends on resource.OrderRemote directly for them, the same
// "services don't depend on other services" rule core/basket.Service
// already applies to CheckoutBasket/CancelBasket.
type Order interface {
	// GetByUserID returns userID's full order history. Mirrors
	// Repository.GetOrdersByUserID at the service layer — used by the
	// source repo's orders handler to render the order-history page.
	GetByUserID(ctx context.Context, userID string) ([]domain.Order, error)
}
