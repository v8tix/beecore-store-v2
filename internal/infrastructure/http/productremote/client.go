// Package productremote is the kawa-based HTTP client implementing
// resource.ProductRemote, ported 1:1 from
// internal/business/core/repository/products_repository.go in the source
// repo (see port/resource.ProductRemote's doc comment for the full
// mapping).
package productremote

import (
	"time"

	"github.com/v8tix/beecore-eda/config"
	stores "github.com/v8tix/beecore-http/messages/stores/v1"
	"github.com/v8tix/kawa"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/core/port/resource"
	"github.com/v8tix/beecore-store-v2/internal/infrastructure/http/httpshared"
)

// Client is the outbound HTTP boundary for the product vertical slice. It
// holds *config.Cfg directly (same as source repo's BaseRepositoryImpl
// struct) — the URLs and shared *http.Client it needs all live there
// already.
type Client struct {
	cfg *config.Cfg
}

var _ resource.ProductRemote = (*Client)(nil)

func NewClient(cfg *config.Cfg) *Client {
	return &Client{cfg: cfg}
}

// deadline and retryPolicy mirror BaseRepositoryImpl.deadline/retryPolicy
// in the source repo: every call here is a single-record or single-page
// lookup, so they use the config-tunable "fast" HTTP client profile —
// unlike order's GetOrdersByUserID (Task 15), no call here needs the
// "slow" profile.
func (c *Client) deadline() time.Duration {
	return httpshared.Deadline(c.cfg)
}

func (c *Client) retryPolicy() kawa.RetryPolicy {
	return httpshared.RetryPolicy(c.cfg)
}

// toDomainProduct maps a stores.GetProductV1Res wire value onto
// domain.Product — a direct field-for-field copy, no renaming or
// reshaping.
func toDomainProduct(p stores.GetProductV1Res) domain.Product {
	return domain.Product{
		ID:                  p.ID,
		StoreID:             p.StoreID,
		Name:                p.Name,
		Description:         p.Description,
		SKU:                 p.SKU,
		Price:               p.Price,
		Category:            p.Category,
		ImgURL:              p.ImgURL,
		VideoURL:            p.VideoURL,
		Status:              p.Status,
		Quantity:            p.Quantity,
		Weight:              p.Weight,
		Width:               p.Width,
		Height:              p.Height,
		Depth:               p.Depth,
		DiscountPercentage:  p.DiscountPercentage,
		DiscountDescription: p.DiscountDescription,
		Subtotal:            p.Subtotal,
		CreatedAt:           p.CreatedAt,
		UpdatedAt:           p.UpdatedAt,
	}
}
