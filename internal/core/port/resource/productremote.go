package resource

import (
	"context"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
)

//go:generate mockery --name ProductRemote

// ProductRemote is the outbound HTTP boundary for the product slice,
// ported from internal/business/core/repository/products_repository.go in
// the source repo. Both methods call the downstream beecore-stores
// service (config.Integration.V1.StoresURL) — despite the "Stores" name,
// that service is where beecore-store's storefront actually gets its
// *product* catalog from; it has no dedicated store-entity lookup/search
// endpoint that this repo calls (see domain.Store's doc comment and Task
// 10's finding for why no separate StoreRemote exists). Every method name
// below matches its source counterpart; signatures gain a leading ctx and
// drop the kawa.Envelope/*_V1Res wire shape in favor of domain types.
type ProductRemote interface {
	// FindProduct looks up a single product by id. Mirrors
	// BaseRepositoryImpl.FindProduct (GET stores/products/{id}).
	FindProduct(ctx context.Context, id, token string) (domain.Product, error)

	// FindPaginatedProducts full-text searches the catalog for searchTerm,
	// returning page's products (zero-based) and the total match count
	// across all pages. Mirrors BaseRepositoryImpl.FindPaginatedProducts,
	// which queries the Stores API's search-count endpoint
	// (products/search/count) and its search endpoint (products/search)
	// separately and pairs the results — no Link-header pagination, unlike
	// beecore-admin-v2's product listing.
	FindPaginatedProducts(ctx context.Context, searchTerm string, page int64, token string) (domain.ProductPage, error)
}
