// Package orderremote is the kawa-based HTTP client implementing
// resource.OrderRemote, ported 1:1 from
// internal/business/core/repository/orders_repository.go in the source
// repo (see port/resource.OrderRemote's doc comment for the full
// mapping).
package orderremote

import (
	"time"

	"github.com/v8tix/beecore-eda/config"
	common "github.com/v8tix/beecore-http/messages/common/v1"
	orders "github.com/v8tix/beecore-http/messages/orders/v1"
	"github.com/v8tix/kawa"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/core/port/resource"
	"github.com/v8tix/beecore-store-v2/internal/infrastructure/http/httpshared"
)

// Client is the outbound HTTP boundary for the order vertical slice. It
// holds *config.Cfg directly (same as source repo's BaseRepositoryImpl
// struct) — the URLs and shared *http.Client it needs all live there
// already.
type Client struct {
	cfg *config.Cfg
}

var _ resource.OrderRemote = (*Client)(nil)

func NewClient(cfg *config.Cfg) *Client {
	return &Client{cfg: cfg}
}

// deadline and retryPolicy mirror BaseRepositoryImpl.deadline/retryPolicy
// in the source repo: CreateOrder/GetOrder/CancelOrder/CompleteOrder/
// ReadyOrder are all single-record mutations or lookups, so they use the
// config-tunable "fast" HTTP client profile. GetOrdersByUserID does NOT
// use these — see its own doc comment and implementation in
// orderremote.go for the "slow" profile override this method must
// preserve exactly.
func (c *Client) deadline() time.Duration {
	return httpshared.Deadline(c.cfg)
}

func (c *Client) retryPolicy() kawa.RetryPolicy {
	return httpshared.RetryPolicy(c.cfg)
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

func toDomainOrderShippingAddress(a common.ShippingAddressV1) domain.OrderShippingAddress {
	return domain.OrderShippingAddress{
		Country:         a.Country,
		State:           a.State,
		City:            a.City,
		PostalCode:      a.PostalCode,
		MainStreet:      a.MainStreet,
		SecondaryStreet: a.SecondaryStreet,
		Numeration:      a.Numeration,
		Phone:           a.Phone,
	}
}

// toDomainOrder maps an orders.GetOrderV1Res wire value onto domain.Order
// — a direct field-for-field copy, no renaming or reshaping.
func toDomainOrder(o orders.GetOrderV1Res) domain.Order {
	return domain.Order{
		ID:              o.OrderID,
		UserID:          o.UserID,
		PaymentID:       o.PaymentID,
		BasketID:        o.BasketID,
		Status:          o.Status,
		ShippingAddress: toDomainOrderShippingAddress(o.ShippingAddress),
		FinancialItem:   toDomainFinancialLineItem(o.FinancialItem),
		UpdatedAt:       o.UpdatedAt,
	}
}

func toWireFinancialItem(f domain.FinancialLineItem) common.FinancialItemV1 {
	products := make([]common.ProductV1, 0, len(f.Products))
	for _, p := range f.Products {
		products = append(products, common.ProductV1{
			ID:          p.ID,
			Name:        p.Name,
			Description: p.Description,
			Img:         p.Img,
			Price:       p.Price,
			Subtotal:    p.Subtotal,
			Quantity:    p.Quantity,
		})
	}

	return common.FinancialItemV1{
		Store: common.StoreV1{
			ID:    f.Store.ID,
			Owner: f.Store.Owner,
			Name:  f.Store.Name,
			Email: f.Store.Email,
		},
		Products:       products,
		ShippingAmount: f.ShippingAmount,
		Taxes:          f.Taxes,
		TaxesAmount:    f.TaxesAmount,
		Subtotal:       f.Subtotal,
		Total:          f.Total,
	}
}
