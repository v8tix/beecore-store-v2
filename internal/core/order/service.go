// Package order is the order use-case, ported from
// internal/business/core/repository/orders_repository.go in the source
// repo at the service layer (see port/service.Order's doc comment for the
// full mapping and its one caller).
package order

import (
	"context"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/core/port/resource"
	"github.com/v8tix/beecore-store-v2/internal/core/port/service"
)

// Dependencies are orderService's collaborators, all resolved once at
// composition-root time rather than read from *config.Cfg here — core
// stays free of any infra/config import.
type Dependencies struct {
	OrderRemote resource.OrderRemote

	// AuthRemote is used only for its GetAdminToken method — reused from
	// the auth slice's resource.AuthRemote rather than duplicated here,
	// same pattern core/basket.Service and core/product.Service use.
	AuthRemote resource.AuthRemote
}

type orderService struct {
	deps Dependencies
}

var _ service.Order = (*orderService)(nil)

func NewOrderService(d Dependencies) *orderService {
	return &orderService{deps: d}
}

// GetByUserID mirrors Repository.GetOrdersByUserID exposed at the service
// layer.
func (s *orderService) GetByUserID(ctx context.Context, userID string) ([]domain.Order, error) {
	at, err := s.deps.AuthRemote.GetAdminToken(ctx)
	if err != nil {
		return nil, err
	}

	return s.deps.OrderRemote.GetOrdersByUserID(ctx, userID, at)
}
