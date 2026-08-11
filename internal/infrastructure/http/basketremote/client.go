// Package basketremote is the kawa-based HTTP client implementing
// resource.BasketRemote, ported 1:1 from
// internal/business/core/repository/baskets_repository.go in the source
// repo (see port/resource.BasketRemote's doc comment for the full
// mapping).
package basketremote

import (
	"fmt"
	"time"

	"github.com/v8tix/beecore-eda/config"
	baskets "github.com/v8tix/beecore-http/messages/baskets/v1"
	common "github.com/v8tix/beecore-http/messages/common/v1"
	"github.com/v8tix/kawa"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/core/port/resource"
	"github.com/v8tix/beecore-store-v2/internal/infrastructure/http/httpshared"
)

// Client is the outbound HTTP boundary for the basket vertical slice. It
// holds *config.Cfg directly (same as source repo's BaseRepositoryImpl
// struct) — the URLs and shared *http.Client it needs all live there
// already.
type Client struct {
	cfg *config.Cfg
}

var _ resource.BasketRemote = (*Client)(nil)

func NewClient(cfg *config.Cfg) *Client {
	return &Client{cfg: cfg}
}

// deadline and retryPolicy mirror BaseRepositoryImpl.deadline/retryPolicy
// in the source repo: every call here is a single-record mutation or
// lookup, so they use the config-tunable "fast" HTTP client profile.
func (c *Client) deadline() time.Duration {
	return httpshared.Deadline(c.cfg)
}

func (c *Client) retryPolicy() kawa.RetryPolicy {
	return httpshared.RetryPolicy(c.cfg)
}

// translateHTTPError turns a kawa.ErrInvalidHTTPStatus into a plain error
// carrying the downstream response body's parsed message, for call sites
// that don't need to distinguish specific downstream rejection reasons.
// Non-HTTP errors (network failures) pass through unchanged.
//
// TODO(httpshared): this diverges from authremote's translateHTTPError —
// see that package's TODO — unifying the two is deferred to its own
// follow-up commit rather than folded into this dedup pass.
func translateHTTPError(err error) error {
	statusCode, message, parseErr, ok := httpshared.DecodeHTTPError(err)
	if !ok {
		return err
	}
	if parseErr != nil {
		return parseErr
	}

	return fmt.Errorf("downstream request failed with status %d: %s", statusCode, message)
}

func toDomainStore(s common.StoreV1) domain.Store {
	return domain.Store{
		ID:    s.ID,
		Owner: s.Owner,
		Name:  s.Name,
		Email: s.Email,
	}
}

func toDomainBasketProduct(p common.ProductV1) domain.BasketProduct {
	return domain.BasketProduct{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		Img:         p.Img,
		Price:       p.Price,
		Subtotal:    p.Subtotal,
		Quantity:    p.Quantity,
	}
}

func toDomainBasketItem(i baskets.GetBasketItemV1Res) domain.BasketItem {
	return domain.BasketItem{
		Store:   toDomainStore(i.Store),
		Product: toDomainBasketProduct(i.Product),
	}
}

// toDomainBasket maps a baskets.GetBasketV1Res wire value onto
// domain.Basket. The wire value doesn't carry the basket's own ID (see
// domain.Basket's doc comment), so id — the input the caller looked the
// basket up by — is copied onto the result directly.
func toDomainBasket(id string, b baskets.GetBasketV1Res) domain.Basket {
	items := make([]domain.BasketItem, 0, len(b.Items))
	for _, i := range b.Items {
		items = append(items, toDomainBasketItem(i))
	}

	return domain.Basket{
		ID:        id,
		UserID:    b.UserID,
		PaymentID: b.PaymentID,
		Items:     items,
		Status:    b.Status,
	}
}

func toDomainFinancialLineItem(f common.FinancialItemV1) domain.FinancialLineItem {
	products := make([]domain.BasketProduct, 0, len(f.Products))
	for _, p := range f.Products {
		products = append(products, toDomainBasketProduct(p))
	}

	return domain.FinancialLineItem{
		Store:          toDomainStore(f.Store),
		Products:       products,
		ShippingAmount: f.ShippingAmount,
		Taxes:          f.Taxes,
		TaxesAmount:    f.TaxesAmount,
		Subtotal:       f.Subtotal,
		Total:          f.Total,
	}
}

func toDomainFinancials(f baskets.GetFinancialItemsV1Res) domain.Financials {
	items := make([]domain.FinancialLineItem, 0, len(f.Items))
	for _, i := range f.Items {
		items = append(items, toDomainFinancialLineItem(i))
	}

	return domain.Financials{
		Items:               items,
		Subtotal:            f.Subtotal,
		TotalShippingAmount: f.TotalShippingAmount,
		TotalTaxesAmount:    f.TotalTaxesAmount,
		Total:               f.Total,
	}
}
