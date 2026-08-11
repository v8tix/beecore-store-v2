package service

import (
	"context"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
)

//go:generate mockery --name Product

// Product is the product use-case, ported from
// internal/business/core/repository/products_repository.go in the source
// repo at the service layer, for the storefront browsing flows that call
// it directly: the source repo's cmd/web/products_handler.go findProducts
// (full-text search + pagination, "/search") and productDetails
// ("/products/{product_id}").
//
// Like core/address.Service, every method here drops the trailing token
// parameter: this service resolves its own admin token via
// AuthRemote.GetAdminToken, the same pattern core/user.Service and
// core/address.Service already use, since every handler that calls this
// service reaches it directly rather than through another use-case that
// already has a token in hand.
type Product interface {
	// FindByID looks up a single product by id. Mirrors
	// Repository.FindProduct at the service layer.
	FindByID(ctx context.Context, id string) (domain.Product, error)

	// Search full-text searches the catalog for searchTerm, returning
	// page's products (zero-based) and the total match count across all
	// pages. Mirrors Repository.FindPaginatedProducts at the service
	// layer.
	Search(ctx context.Context, searchTerm string, page int64) (domain.ProductPage, error)
}
