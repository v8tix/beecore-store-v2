package orders

import (
	"time"

	"github.com/v8tix/beecore-http/messages/common/v1"
	"github.com/v8tix/beecore-http/validator"
)

type (
	CreateOrderV1Req struct {
		UserID            string                 `json:"user_id,omitempty"`
		PaymentID         string                 `json:"payment_id,omitempty"`
		BasketID          string                 `json:"basket_id,omitempty"`
		ShippingAddressID string                 `json:"shipping_address_id,omitempty"`
		FinancialItem     common.FinancialItemV1 `json:"financial_item,omitempty"`
	}
	CreateOrderV1Res struct {
		OrderID string `json:"order_id,omitempty"`
	}

	CreateOrderEnvRes struct {
		Order CreateOrderV1Res `json:"order,omitempty"`
	}

	CompleteOrderV1Req struct {
		InvoiceID string `json:"invoice_id,omitempty"`
	}
	GetOrderV1Res struct {
		OrderID         string                   `json:"order_id,omitempty"`
		UserID          string                   `json:"user_id,omitempty"`
		PaymentID       string                   `json:"payment_id,omitempty"`
		BasketID        string                   `json:"basket_id,omitempty"`
		Status          string                   `json:"status,omitempty"`
		ShippingAddress common.ShippingAddressV1 `json:"shipping_address,omitempty"`
		FinancialItem   common.FinancialItemV1   `json:"financial_item,omitempty"`
		UpdatedAt       time.Time                `json:"updated_at,omitempty"`
	}

	GetOrderEnvV1Res struct {
		Order GetOrderV1Res `json:"order,omitempty"`
	}
	GetOrdersByUserIDEnvV1Res struct {
		Orders []GetOrderV1Res `json:"orders,omitempty"`
	}
)

func (c CreateOrderV1Req) Req() {}

func (c CreateOrderV1Req) Validate(v *validator.Validator) {
	v.Check(len(c.FinancialItem.Products) != 0, "items", "can't be empty")
	v.Check(len(c.UserID) != 0, "user_id", "must be provided")
	v.Check(len(c.PaymentID) != 0, "payment_id", "must be provided")
	v.Check(len(c.BasketID) != 0, "basket_id", "must be provided")
	v.Check(len(c.ShippingAddressID) != 0, "shipping_address_id", "must be provided")
}

func (c CreateOrderV1Res) Res() {}

func (c CompleteOrderV1Req) Req() {}

func (c CompleteOrderV1Req) Validate(v *validator.Validator) {
	v.Check(len(c.InvoiceID) != 0, "invoice_id", "must be provided")
}

func (c CreateOrderEnvRes) Res() {}

func (g GetOrderEnvV1Res) Res() {}

func (g GetOrdersByUserIDEnvV1Res) Res() {}
