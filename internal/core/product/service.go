// Package product is the product use-case, ported from
// internal/business/core/repository/products_repository.go in the source
// repo at the service layer (see port/service.Product's doc comment for
// the full mapping and its callers). It's a thin pass-through to
// resource.ProductRemote — products_repository.go's methods have no
// orchestration logic of their own in the source repo — resolving its own
// admin token via resource.AuthRemote first, the same pattern
// core/address.Service uses.
package product

import (
	"context"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/core/port/resource"
	"github.com/v8tix/beecore-store-v2/internal/core/port/service"
)

// Dependencies are productService's collaborators, all resolved once at
// composition-root time rather than read from *config.Cfg here — core
// stays free of any infra/config import.
type Dependencies struct {
	ProductRemote resource.ProductRemote

	// AuthRemote is used only for its GetAdminToken method — reused from
	// the auth slice's resource.AuthRemote rather than duplicated here.
	AuthRemote resource.AuthRemote
}

type productService struct {
	deps Dependencies
}

var _ service.Product = (*productService)(nil)

func NewProductService(d Dependencies) *productService {
	return &productService{deps: d}
}

// FindByID mirrors Repository.FindProduct exposed at the service layer.
func (s *productService) FindByID(ctx context.Context, id string) (domain.Product, error) {
	at, err := s.deps.AuthRemote.GetAdminToken(ctx)
	if err != nil {
		return domain.Product{}, err
	}

	return s.deps.ProductRemote.FindProduct(ctx, id, at)
}

// Search mirrors Repository.FindPaginatedProducts exposed at the service
// layer.
func (s *productService) Search(ctx context.Context, searchTerm string, page int64) (domain.ProductPage, error) {
	at, err := s.deps.AuthRemote.GetAdminToken(ctx)
	if err != nil {
		return domain.ProductPage{}, err
	}

	return s.deps.ProductRemote.FindPaginatedProducts(ctx, searchTerm, page, at)
}
