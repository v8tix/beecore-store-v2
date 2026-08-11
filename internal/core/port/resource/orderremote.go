package resource

import (
	"context"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
)

//go:generate mockery --name OrderRemote

// OrderRemote is the outbound HTTP boundary for the order slice, ported
// from internal/business/core/repository/orders_repository.go in the
// source repo. Every method name below matches its source counterpart;
// signatures gain a leading ctx and drop the
// kawa.Envelope/orders.*_V1Req/*_V1Res wire shape in favor of domain types
// or primitives — translating a downstream response into a plain Go error
// is this adapter's job, so core/order and its handler never need to know
// kawa exists.
//
// Only GetOrdersByUserID and CreateOrder have a live caller in this repo
// today (order_handler.go's orders history page, and
// payphone_handlers.go's PayPhoneConfirm — the latter a Payment-slice
// concern, plan Task 17/18). GetOrder/CancelOrder/CompleteOrder/ReadyOrder
// are ported anyway for full parity with orders_repository.go's method
// set, same precedent as resource.BasketRemote's CheckoutBasket/
// CancelBasket — a future caller (e.g. an order-management admin flow, or
// the Payment slice's own cancellation path) depends on this interface
// directly rather than requiring a second port later.
type OrderRemote interface {
	// CreateOrder creates one order for req.FinancialItem's store. Mirrors
	// BaseRepositoryImpl.CreateOrder (POST orders). Called once per store
	// by the source repo's PayPhoneConfirm handler
	// (dto.ToCreateOrdersV1Req builds one req per financial-item/store);
	// not yet called from this repo's core/order (Payment slice, plan
	// Task 17/18) but ported here for full parity.
	CreateOrder(ctx context.Context, req domain.CreateOrderRequest, token string) (orderID string, err error)

	// GetOrder looks up a single order by ID. Mirrors
	// BaseRepositoryImpl.GetOrder (GET orders/{id}). No live caller in this
	// repo today — ported for full parity, see this interface's doc
	// comment.
	GetOrder(ctx context.Context, orderID, token string) (domain.Order, error)

	// GetOrdersByUserID returns userID's full order history. Mirrors
	// BaseRepositoryImpl.GetOrdersByUserID (GET orders/all/users/{id}) —
	// used by the source repo's orders handler ("/orders") to render the
	// order-history page.
	//
	// Unlike every other call in this package, a user's full order
	// history isn't the simple, low-latency lookup the "fast" profile is
	// tuned for: the source repo's implementation deliberately overrides
	// it with the "slow" HTTP client profile's larger deadline/retry
	// budget (10s/3/500ms defaults vs. fast's 1s/3/100ms — both
	// config-tunable via config.HTTPClient). The adapter implementing
	// this method must preserve that override exactly, not the "fast"
	// profile every other method here uses.
	GetOrdersByUserID(ctx context.Context, userID, token string) ([]domain.Order, error)

	// CancelOrder cancels orderID. Mirrors BaseRepositoryImpl.CancelOrder
	// (DELETE orders/{id}). No live caller in this repo today — ported for
	// full parity, see this interface's doc comment.
	CancelOrder(ctx context.Context, orderID, token string) error

	// CompleteOrder marks orderID complete, recording invoiceID. Mirrors
	// BaseRepositoryImpl.CompleteOrder (PUT orders/{id}). No live caller
	// in this repo today — ported for full parity, see this interface's
	// doc comment.
	CompleteOrder(ctx context.Context, orderID, invoiceID, token string) error

	// ReadyOrder marks orderID ready. Mirrors
	// BaseRepositoryImpl.ReadyOrder (PUT orders/{id}/ready). No live
	// caller in this repo today — ported for full parity, see this
	// interface's doc comment.
	ReadyOrder(ctx context.Context, orderID, token string) error
}
