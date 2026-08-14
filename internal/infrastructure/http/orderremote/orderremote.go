package orderremote

import (
	"context"
	"fmt"

	orders "github.com/v8tix/beecore-http/messages/orders/v1"
	"github.com/v8tix/kawa"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/infrastructure/http/httpshared"
)

// CreateOrder mirrors BaseRepositoryImpl.CreateOrder: POST orders.
//
// Deliberately a single-shot call (.Do, not .DoWithRetry) — confirmed the
// hard way that retrying it duplicates orders: this endpoint has no
// idempotency key, and beecore-ordering's own CreateOrder handler
// (domain-event publish + immediate GetOrder re-fetch) routinely takes
// longer than the "fast" HTTP profile's 1s deadline under real load. A
// client-side timeout doesn't mean the server-side write failed — it may
// complete moments later — so retrying a POST here risks creating a
// second, fully persisted order for the same checkout. If CreateOrder
// itself times out, that needs to surface as a real error the caller
// (ConfirmPayment) can act on, not be silently retried into a duplicate.
//
// Also borrows the "slow" profile's deadline (same precedent as
// GetOrdersByUserID below) rather than c.deadline()'s "fast" 1s — that's
// what was too tight to begin with (observed CreateOrder taking up to
// ~2.1s server-side); a single attempt still needs a realistic deadline,
// just not a retry on top of it.
func (c *Client) CreateOrder(ctx context.Context, req domain.CreateOrderRequest, token string) (string, error) {
	url := fmt.Sprintf("%s/%s", c.cfg.Integration.V1.OrdersURL, "orders")
	wireReq := orders.CreateOrderV1Req{
		UserID:            req.UserID,
		PaymentID:         req.PaymentID,
		BasketID:          req.BasketID,
		ShippingAddressID: req.ShippingAddressID,
		FinancialItem:     toWireFinancialItem(req.FinancialItem),
	}

	call := kawa.NewCall[orders.CreateOrderV1Req, orders.GetOrderEnvV1Res](c.cfg.Web.HTTPClient, kawa.Post, url).
		WithHeaders(httpshared.BuildAuthHeader(token)).
		WithDeadline(c.cfg.HTTPClient.GetSlow().Timeout.Duration())

	env, err := call.Do(ctx, &wireReq)
	if err != nil {
		return "", err
	}

	if env.Body == nil {
		return "", nil
	}

	return env.Body.Order.OrderID, nil
}

// GetOrder mirrors BaseRepositoryImpl.GetOrder: GET orders/{id}.
func (c *Client) GetOrder(ctx context.Context, orderID, token string) (domain.Order, error) {
	url := fmt.Sprintf("%s/%s/%s", c.cfg.Integration.V1.OrdersURL, "orders", orderID)

	call := kawa.NewCall[kawa.NoReq, orders.GetOrderEnvV1Res](c.cfg.Web.HTTPClient, kawa.Get, url).
		WithHeaders(httpshared.BuildAuthHeader(token)).
		WithDeadline(c.deadline()).
		WithRetryPolicy(c.retryPolicy())

	env, err := call.DoWithRetry(ctx, nil)
	if err != nil {
		return domain.Order{}, err
	}

	if env.Body == nil {
		return domain.Order{}, nil
	}

	return toDomainOrder(env.Body.Order), nil
}

// GetOrdersByUserID mirrors BaseRepositoryImpl.GetOrdersByUserID: GET
// orders/all/users/{id}.
//
// A user's full order history isn't the simple, low-latency lookup the
// "fast" profile is tuned for — this deliberately uses the "slow"
// profile's larger deadline/retry budget instead, exactly like the
// source. Every other method in this package uses c.deadline()/
// c.retryPolicy() (the "fast" profile via Client's shared helpers); this
// is the one call site in this repo that must not.
func (c *Client) GetOrdersByUserID(ctx context.Context, userID, token string) ([]domain.Order, error) {
	url := fmt.Sprintf("%s/%s/%s/%s/%s", c.cfg.Integration.V1.OrdersURL, "orders", "all", "users", userID)

	profile := c.cfg.HTTPClient.GetSlow()
	call := kawa.NewCall[kawa.NoReq, orders.GetOrdersByUserIDEnvV1Res](c.cfg.Web.HTTPClient, kawa.Get, url).
		WithHeaders(httpshared.BuildAuthHeader(token)).
		WithDeadline(profile.Timeout.Duration()).
		WithRetryPolicy(kawa.NewExponentialRetryPolicy(profile.MaxRetries, profile.RetryInterval.Duration()))

	env, err := call.DoWithRetry(ctx, nil)
	if err != nil {
		return nil, err
	}

	if env.Body == nil {
		return nil, nil
	}

	result := make([]domain.Order, 0, len(env.Body.Orders))
	for _, o := range env.Body.Orders {
		result = append(result, toDomainOrder(o))
	}

	return result, nil
}

// CancelOrder mirrors BaseRepositoryImpl.CancelOrder: DELETE orders/{id}.
func (c *Client) CancelOrder(ctx context.Context, orderID, token string) error {
	url := fmt.Sprintf("%s/%s/%s", c.cfg.Integration.V1.OrdersURL, "orders", orderID)

	call := kawa.NewCall[kawa.NoReq, kawa.NoRes](c.cfg.Web.HTTPClient, kawa.Delete, url).
		WithHeaders(httpshared.BuildAuthHeader(token)).
		WithDeadline(c.deadline()).
		WithRetryPolicy(c.retryPolicy())

	_, err := call.DoWithRetry(ctx, nil)
	return err
}

// CompleteOrder mirrors BaseRepositoryImpl.CompleteOrder: PUT orders/{id}.
func (c *Client) CompleteOrder(ctx context.Context, orderID, invoiceID, token string) error {
	url := fmt.Sprintf("%s/%s/%s", c.cfg.Integration.V1.OrdersURL, "orders", orderID)
	req := orders.CompleteOrderV1Req{InvoiceID: invoiceID}

	call := kawa.NewCall[orders.CompleteOrderV1Req, kawa.NoRes](c.cfg.Web.HTTPClient, kawa.Put, url).
		WithHeaders(httpshared.BuildAuthHeader(token)).
		WithDeadline(c.deadline()).
		WithRetryPolicy(c.retryPolicy())

	_, err := call.DoWithRetry(ctx, &req)
	return err
}

// ReadyOrder mirrors BaseRepositoryImpl.ReadyOrder: PUT orders/{id}/ready.
func (c *Client) ReadyOrder(ctx context.Context, orderID, token string) error {
	url := fmt.Sprintf("%s/%s/%s/%s", c.cfg.Integration.V1.OrdersURL, "orders", orderID, "ready")

	call := kawa.NewCall[kawa.NoReq, kawa.NoRes](c.cfg.Web.HTTPClient, kawa.Put, url).
		WithHeaders(httpshared.BuildAuthHeader(token)).
		WithDeadline(c.deadline()).
		WithRetryPolicy(c.retryPolicy())

	_, err := call.DoWithRetry(ctx, nil)
	return err
}
