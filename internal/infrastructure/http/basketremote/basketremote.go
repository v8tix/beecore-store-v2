package basketremote

import (
	"context"
	"fmt"
	"net/http"

	baskets "github.com/v8tix/beecore-http/messages/baskets/v1"
	"github.com/v8tix/kawa"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/infrastructure/http/httpshared"
)

// CreateBasket mirrors BaseRepositoryImpl.CreateBasket: POST baskets.
//
// Single-shot (.Do, not .DoWithRetry) — same reasoning as
// orderremote.CreateOrder: a POST with no idempotency key that persists a
// new record. A retried timeout here would silently start two baskets for
// one user, one of them orphaned — same failure class as the duplicate
// orders that motivated this fix, just for basket creation instead of
// order creation.
func (c *Client) CreateBasket(ctx context.Context, userID, token string) (string, error) {
	url := fmt.Sprintf("%s/%s", c.cfg.Integration.V1.BasketsURL, "baskets")
	req := baskets.StartBasketV1Req{UserID: userID}

	call := kawa.NewCall[baskets.StartBasketV1Req, baskets.StartBasketEnvV1Res](c.cfg.Web.HTTPClient, kawa.Post, url).
		WithHeaders(httpshared.BuildAuthHeader(token)).
		WithDeadline(c.deadline())

	env, err := call.Do(ctx, &req)
	if err != nil {
		return "", httpshared.TranslateHTTPError(err)
	}

	if env.Body == nil {
		return "", nil
	}

	return env.Body.Basket.BasketID, nil
}

// FindBasket mirrors BaseRepositoryImpl.FindBasket: GET baskets/{id}.
func (c *Client) FindBasket(ctx context.Context, id, token string) (domain.Basket, error) {
	url := fmt.Sprintf("%s/%s/%s", c.cfg.Integration.V1.BasketsURL, "baskets", id)

	call := kawa.NewCall[kawa.NoReq, baskets.GetBasketEnvV1Res](c.cfg.Web.HTTPClient, kawa.Get, url).
		WithHeaders(httpshared.BuildAuthHeader(token)).
		WithDeadline(c.deadline()).
		WithRetryPolicy(c.retryPolicy())

	env, err := call.DoWithRetry(ctx, nil)
	if err != nil {
		return domain.Basket{}, httpshared.TranslateHTTPError(err)
	}

	if env.Body == nil {
		return domain.Basket{}, nil
	}

	return toDomainBasket(id, env.Body.GetBasketV1Res), nil
}

// BasketAddItem mirrors BaseRepositoryImpl.BasketAddItem: PUT
// baskets/{id}/addItem. err is domain.ErrBasketCheckedOut on a downstream
// 409 Conflict — see BasketRemote's doc comment.
func (c *Client) BasketAddItem(ctx context.Context, id, productID string, quantity int, token string) error {
	url := fmt.Sprintf("%s/%s/%s/%s", c.cfg.Integration.V1.BasketsURL, "baskets", id, "addItem")
	req := baskets.AddOrRemoveItemV1Req{ProductID: productID, Quantity: quantity}

	call := kawa.NewCall[baskets.AddOrRemoveItemV1Req, kawa.NoRes](c.cfg.Web.HTTPClient, kawa.Put, url).
		WithHeaders(httpshared.BuildAuthHeader(token)).
		WithDeadline(c.deadline()).
		WithRetryPolicy(c.retryPolicy())

	_, err := call.DoWithRetry(ctx, &req)
	if err != nil {
		statusCode, _, _, ok := httpshared.DecodeHTTPError(err)
		if ok && statusCode == http.StatusConflict {
			return domain.ErrBasketCheckedOut
		}

		return httpshared.TranslateHTTPError(err)
	}

	return nil
}

// BasketRemoveItem mirrors BaseRepositoryImpl.BasketRemoveItem: PUT
// baskets/{id}/removeItem.
func (c *Client) BasketRemoveItem(ctx context.Context, id, productID string, quantity int, token string) error {
	url := fmt.Sprintf("%s/%s/%s/%s", c.cfg.Integration.V1.BasketsURL, "baskets", id, "removeItem")
	req := baskets.AddOrRemoveItemV1Req{ProductID: productID, Quantity: quantity}

	call := kawa.NewCall[baskets.AddOrRemoveItemV1Req, kawa.NoRes](c.cfg.Web.HTTPClient, kawa.Put, url).
		WithHeaders(httpshared.BuildAuthHeader(token)).
		WithDeadline(c.deadline()).
		WithRetryPolicy(c.retryPolicy())

	_, err := call.DoWithRetry(ctx, &req)
	if err != nil {
		return httpshared.TranslateHTTPError(err)
	}

	return nil
}

// CheckoutBasket mirrors BaseRepositoryImpl.CheckoutBasket: PUT
// baskets/{id}/checkout.
//
// Single-shot (.Do, not .DoWithRetry) — same reasoning as
// orderremote.CreateOrder. Checking out a basket transitions it to
// CHECKED_OUT and fires a domain event downstream services react to; a
// retried timeout risks a second checkout attempt racing that transition
// (best case a harmless no-op/conflict, worst case duplicate downstream
// side effects) instead of a clean error ConfirmPayment's caller can act
// on.
func (c *Client) CheckoutBasket(ctx context.Context, id, paymentID, shippingAddressID, token string) error {
	url := fmt.Sprintf("%s/%s/%s/%s", c.cfg.Integration.V1.BasketsURL, "baskets", id, "checkout")
	req := baskets.CheckoutBasketV1Req{PaymentID: paymentID, ShippingAddressID: shippingAddressID}

	call := kawa.NewCall[baskets.CheckoutBasketV1Req, kawa.NoRes](c.cfg.Web.HTTPClient, kawa.Put, url).
		WithHeaders(httpshared.BuildAuthHeader(token)).
		WithDeadline(c.deadline())

	_, err := call.Do(ctx, &req)
	if err != nil {
		return httpshared.TranslateHTTPError(err)
	}

	return nil
}

// CancelBasket mirrors BaseRepositoryImpl.CancelBasket: DELETE
// baskets/{id}.
func (c *Client) CancelBasket(ctx context.Context, id, token string) error {
	url := fmt.Sprintf("%s/%s/%s", c.cfg.Integration.V1.BasketsURL, "baskets", id)

	call := kawa.NewCall[kawa.NoReq, kawa.NoRes](c.cfg.Web.HTTPClient, kawa.Delete, url).
		WithHeaders(httpshared.BuildAuthHeader(token)).
		WithDeadline(c.deadline()).
		WithRetryPolicy(c.retryPolicy())

	_, err := call.DoWithRetry(ctx, nil)
	if err != nil {
		return httpshared.TranslateHTTPError(err)
	}

	return nil
}

// FindBasketByUserID mirrors BaseRepositoryImpl.FindBasketByUserID: GET
// baskets/opened/users/{id}.
func (c *Client) FindBasketByUserID(ctx context.Context, userID, token string) (string, error) {
	url := fmt.Sprintf("%s/%s/%s/%s/%s", c.cfg.Integration.V1.BasketsURL, "baskets", "opened", "users", userID)

	call := kawa.NewCall[kawa.NoReq, baskets.GetBasketByUserIDEnvV1Res](c.cfg.Web.HTTPClient, kawa.Get, url).
		WithHeaders(httpshared.BuildAuthHeader(token)).
		WithDeadline(c.deadline()).
		WithRetryPolicy(c.retryPolicy())

	env, err := call.DoWithRetry(ctx, nil)
	if err != nil {
		return "", httpshared.TranslateHTTPError(err)
	}

	if env.Body == nil {
		return "", nil
	}

	return env.Body.Basket.BasketID, nil
}

// ComputeFinancials mirrors BaseRepositoryImpl.ComputeFinancials: GET
// baskets/{id}/financials.
func (c *Client) ComputeFinancials(ctx context.Context, id, token string) (domain.Financials, error) {
	url := fmt.Sprintf("%s/%s/%s/%s", c.cfg.Integration.V1.BasketsURL, "baskets", id, "financials")

	call := kawa.NewCall[kawa.NoReq, baskets.GetFinancialItemsEnvV1Res](c.cfg.Web.HTTPClient, kawa.Get, url).
		WithHeaders(httpshared.BuildAuthHeader(token)).
		WithDeadline(c.deadline()).
		WithRetryPolicy(c.retryPolicy())

	env, err := call.DoWithRetry(ctx, nil)
	if err != nil {
		return domain.Financials{}, httpshared.TranslateHTTPError(err)
	}

	if env.Body == nil {
		return domain.Financials{}, nil
	}

	return toDomainFinancials(env.Body.FinancialItems), nil
}
