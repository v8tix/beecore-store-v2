// Package baskets holds the wire-message DTOs for the baskets HTTP API.
package baskets

import (
	"github.com/v8tix/beecore-http/messages/common/v1"
	"github.com/v8tix/beecore-http/validator"
)

type (
	// StartBasketV1Req is the request to start a new basket for a user.
	StartBasketV1Req struct {
		UserID string `json:"user_id,omitempty"`
	}

	// StartBasketV1Res is the response payload identifying a newly started basket.
	StartBasketV1Res struct {
		BasketID string `json:"basket_id,omitempty"`
	}

	// GetBasketByIDV1Res is the response payload identifying a basket looked up by ID.
	GetBasketByIDV1Res struct {
		BasketID string `json:"basket_id,omitempty"`
	}

	// StartBasketEnvV1Res envelopes StartBasketV1Res.
	StartBasketEnvV1Res struct {
		Basket StartBasketV1Res `json:"basket,omitempty"`
	}

	// CheckoutBasketV1Req is the request to check out a basket.
	CheckoutBasketV1Req struct {
		PaymentID         string `json:"payment_id,omitempty"`
		ShippingAddressID string `json:"shipping_address_id,omitempty"`
	}

	// AddOrRemoveItemV1Req is the request to add or remove an item from a basket.
	AddOrRemoveItemV1Req struct {
		ProductID string `json:"product_id,omitempty"`
		Quantity  int    `json:"quantity,omitempty"`
	}

	// GetBasketItemV1Res is a single basket item's store and product snapshot.
	GetBasketItemV1Res struct {
		Store   common.StoreV1   `json:"store,omitempty"`
		Product common.ProductV1 `json:"product,omitempty"`
	}

	// GetFinancialItemsV1Res is the financial breakdown of a basket.
	GetFinancialItemsV1Res struct {
		Items               []common.FinancialItemV1 `json:"financial_items,omitempty"`
		Subtotal            float64                  `json:"subtotal,omitempty"`
		TotalShippingAmount float64                  `json:"total_shipping,omitempty"`
		TotalTaxesAmount    float64                  `json:"total_taxes,omitempty"`
		Total               float64                  `json:"total,omitempty"`
	}

	// GetFinancialItemsEnvV1Res envelopes GetFinancialItemsV1Res.
	GetFinancialItemsEnvV1Res struct {
		FinancialItems GetFinancialItemsV1Res `json:"financial,omitempty"`
	}

	// GetBasketV1Res is a basket's full state.
	GetBasketV1Res struct {
		UserID    string               `json:"user_id,omitempty"`
		PaymentID string               `json:"payment_id,omitempty"`
		Items     []GetBasketItemV1Res `json:"items,omitempty"`
		Status    string               `json:"status,omitempty"`
	}

	// GetBasketEnvV1Res envelopes GetBasketV1Res.
	GetBasketEnvV1Res struct {
		GetBasketV1Res GetBasketV1Res `json:"basket,omitempty"`
	}

	// GetBasketByUserIDEnvV1Res envelopes GetBasketByIDV1Res.
	GetBasketByUserIDEnvV1Res struct {
		Basket GetBasketByIDV1Res `json:"basket,omitempty"`
	}
)

// Req marks StartBasketV1Req as a request payload.
func (s StartBasketV1Req) Req() {}

// Validate checks that StartBasketV1Req's required fields are present.
func (s StartBasketV1Req) Validate(v *validator.Validator) {
	v.Check(len(s.UserID) != 0, "user_id", "must be provided")
}

// Req marks CheckoutBasketV1Req as a request payload.
func (c CheckoutBasketV1Req) Req() {}

// Validate checks that CheckoutBasketV1Req's required fields are present.
func (c CheckoutBasketV1Req) Validate(v *validator.Validator) {
	v.Check(len(c.PaymentID) != 0, "payment_id", "must be provided")
	v.Check(len(c.ShippingAddressID) != 0, "shipping_address_id", "must be provided")
}

// Req marks AddOrRemoveItemV1Req as a request payload.
func (a AddOrRemoveItemV1Req) Req() {}

// Validate checks that AddOrRemoveItemV1Req's required fields are present.
func (a AddOrRemoveItemV1Req) Validate(v *validator.Validator) {
	v.Check(len(a.ProductID) != 0, "product_id", "must be provided")
}

// Res marks StartBasketEnvV1Res as a response payload.
func (s StartBasketEnvV1Res) Res() {}

// Res marks GetBasketEnvV1Res as a response payload.
func (g GetBasketEnvV1Res) Res() {}

// Res marks GetBasketByUserIDEnvV1Res as a response payload.
func (g GetBasketByUserIDEnvV1Res) Res() {}

// Res marks GetFinancialItemsEnvV1Res as a response payload.
func (g GetFinancialItemsEnvV1Res) Res() {}
