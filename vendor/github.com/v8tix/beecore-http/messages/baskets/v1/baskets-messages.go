package baskets

import (
	"github.com/v8tix/beecore-http/messages/common/v1"
	"github.com/v8tix/beecore-http/validator"
)

type (
	StartBasketV1Req struct {
		UserID string `json:"user_id,omitempty"`
	}

	StartBasketV1Res struct {
		BasketID string `json:"basket_id,omitempty"`
	}

	GetBasketByIDV1Res struct {
		BasketID string `json:"basket_id,omitempty"`
	}

	StartBasketEnvV1Res struct {
		Basket StartBasketV1Res `json:"basket,omitempty"`
	}

	CheckoutBasketV1Req struct {
		PaymentID         string `json:"payment_id,omitempty"`
		ShippingAddressID string `json:"shipping_address_id,omitempty"`
	}

	AddOrRemoveItemV1Req struct {
		ProductID string `json:"product_id,omitempty"`
		Quantity  int    `json:"quantity,omitempty"`
	}

	GetBasketItemV1Res struct {
		Store   common.StoreV1   `json:"store,omitempty"`
		Product common.ProductV1 `json:"product,omitempty"`
	}
	GetFinancialItemsV1Res struct {
		Items               []common.FinancialItemV1 `json:"financial_items,omitempty"`
		Subtotal            float64                  `json:"subtotal,omitempty"`
		TotalShippingAmount float64                  `json:"total_shipping,omitempty"`
		TotalTaxesAmount    float64                  `json:"total_taxes,omitempty"`
		Total               float64                  `json:"total,omitempty"`
	}

	GetFinancialItemsEnvV1Res struct {
		FinancialItems GetFinancialItemsV1Res `json:"financial,omitempty"`
	}

	GetBasketV1Res struct {
		UserID    string               `json:"user_id,omitempty"`
		PaymentID string               `json:"payment_id,omitempty"`
		Items     []GetBasketItemV1Res `json:"items,omitempty"`
		Status    string               `json:"status,omitempty"`
	}

	GetBasketEnvV1Res struct {
		GetBasketV1Res GetBasketV1Res `json:"basket,omitempty"`
	}

	GetBasketByUserIDEnvV1Res struct {
		Basket GetBasketByIDV1Res `json:"basket,omitempty"`
	}
)

func (s StartBasketV1Req) Req() {}

func (s StartBasketV1Req) Validate(v *validator.Validator) {
	v.Check(len(s.UserID) != 0, "user_id", "must be provided")
}

func (c CheckoutBasketV1Req) Req() {}

func (c CheckoutBasketV1Req) Validate(v *validator.Validator) {
	v.Check(len(c.PaymentID) != 0, "payment_id", "must be provided")
	v.Check(len(c.ShippingAddressID) != 0, "shipping_address_id", "must be provided")
}

func (a AddOrRemoveItemV1Req) Req() {}

func (a AddOrRemoveItemV1Req) Validate(v *validator.Validator) {
	v.Check(len(a.ProductID) != 0, "product_id", "must be provided")
}

func (s StartBasketEnvV1Res) Res() {}

func (g GetBasketEnvV1Res) Res() {}

func (g GetBasketByUserIDEnvV1Res) Res() {}
func (g GetFinancialItemsEnvV1Res) Res() {}
