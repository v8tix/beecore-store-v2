// Package payments holds the wire-message DTOs for the payments HTTP API.
package payments

import (
	"time"

	"github.com/v8tix/beecore-http/validator"
)

type (
	// CreatePaymentV1Req is the request to create a payment.
	CreatePaymentV1Req struct {
		PaymentID string  `json:"payment_id,omitempty"`
		UserID    string  `json:"user_id,omitempty"`
		Amount    float64 `json:"amount,omitempty"`
		Platform  string  `json:"platform,omitempty"`
		Status    string  `json:"status,omitempty"`
		Request   string  `json:"request,omitempty"`
		Response  string  `json:"response,omitempty"`
	}

	// UpdatePaymentV1Req is the request to update a payment's status/request/response.
	UpdatePaymentV1Req struct {
		Status   string `json:"status,omitempty"`
		Request  string `json:"request,omitempty"`
		Response string `json:"response,omitempty"`
	}

	// PaymentV1Res is a payment's full state.
	PaymentV1Res struct {
		PaymentID string    `json:"payment_id,omitempty"`
		UserID    string    `json:"user_id,omitempty"`
		Amount    float64   `json:"amount,omitempty"`
		Platform  string    `json:"platform,omitempty"`
		Status    string    `json:"status,omitempty"`
		Request   string    `json:"request,omitempty"`
		Response  string    `json:"response,omitempty"`
		CreatedAt time.Time `json:"created_at,omitempty"`
		UpdatedAt time.Time `json:"updated_at,omitempty"`
	}

	// PaymentEnvV1Res envelopes PaymentV1Res.
	PaymentEnvV1Res struct {
		Payment PaymentV1Res `json:"payment,omitempty"`
	}

	// UpdatePaymentStatusV1Req is the request to update a payment's status.
	UpdatePaymentStatusV1Req struct {
		Status string `json:"status,omitempty"`
	}

	// PaymentsEnvV1Res envelopes a list of PaymentV1Res.
	//
	//nolint:revive // stutters as payments.PaymentsEnvV1Res, but this is a wire DTO imported
	// by name from other repos (e.g. beecore-admin-v2); renaming it is a breaking API change.
	PaymentsEnvV1Res struct {
		Payments []PaymentV1Res `json:"payments,omitempty"`
	}

	// CreatePaymentV1Res is the response payload identifying a newly created payment.
	CreatePaymentV1Res struct {
		PaymentID string `json:"payment_id,omitempty"`
	}

	// CreatePaymentEnvV1Res envelopes CreatePaymentV1Res.
	CreatePaymentEnvV1Res struct {
		CreatePaymentV1Res CreatePaymentV1Res `json:"payment,omitempty"`
	}

	// CreateInvoiceV1Req is the request to create an invoice for an order.
	CreateInvoiceV1Req struct {
		OrderID   string  `json:"order_id,omitempty"`
		PaymentID string  `json:"payment_id,omitempty"`
		Amount    float64 `json:"amount,omitempty"`
	}

	// CreateInvoiceV1Res is the response payload identifying a newly created invoice.
	CreateInvoiceV1Res struct {
		InvoiceID string `json:"invoice_id,omitempty"`
	}

	// AdjustInvoiceV1Req is the request to adjust an invoice's amount.
	AdjustInvoiceV1Req struct {
		Amount float64 `json:"amount,omitempty"`
	}
)

// Req marks CreatePaymentV1Req as a request payload.
func (a CreatePaymentV1Req) Req() {}

// Validate checks that CreatePaymentV1Req's required fields are present and valid.
func (a CreatePaymentV1Req) Validate(v *validator.Validator) {
	v.Check(len(a.PaymentID) != 0, "payment_id", "must be provided")
	v.Check(len(a.UserID) != 0, "user_id", "must be provided")
	v.Check(len(a.Platform) != 0, "platform", "must be provided")
	v.Check(len(a.Status) != 0, "status", "must be provided")
	v.Check(a.Amount > 0, "amount", "must be greater than zero")
}

// Res marks CreatePaymentV1Res as a response payload.
func (c CreatePaymentV1Res) Res() {}

// Req marks CreateInvoiceV1Req as a request payload.
func (c CreateInvoiceV1Req) Req() {}

// Validate checks that CreateInvoiceV1Req's required fields are present and valid.
func (c CreateInvoiceV1Req) Validate(v *validator.Validator) {
	v.Check(len(c.OrderID) != 0, "order_id", "must be provided")
	v.Check(len(c.PaymentID) != 0, "payment_id", "must be provided")
	v.Check(c.Amount > 0, "amount", "must be greater than zero")
}

// Req marks UpdatePaymentStatusV1Req as a request payload.
func (u UpdatePaymentStatusV1Req) Req() {}

// Validate checks that UpdatePaymentStatusV1Req's required fields are present.
func (u UpdatePaymentStatusV1Req) Validate(v *validator.Validator) {
	v.Check(len(u.Status) != 0, "status", "must be provided")
}

// Res marks CreateInvoiceV1Res as a response payload.
func (c CreateInvoiceV1Res) Res() {}

// Res marks PaymentsEnvV1Res as a response payload.
func (p PaymentsEnvV1Res) Res() {}

// Res marks PaymentEnvV1Res as a response payload.
func (p PaymentEnvV1Res) Res() {}

// Res marks CreatePaymentEnvV1Res as a response payload.
func (c CreatePaymentEnvV1Res) Res() {}

// Req marks AdjustInvoiceV1Req as a request payload.
func (a AdjustInvoiceV1Req) Req() {}

// Validate checks that AdjustInvoiceV1Req's required fields are present and valid.
func (a AdjustInvoiceV1Req) Validate(v *validator.Validator) {
	v.Check(a.Amount > 0, "amount", "must be greater than zero")
}

// Req marks UpdatePaymentV1Req as a request payload.
func (u UpdatePaymentV1Req) Req() {}

// Validate checks that UpdatePaymentV1Req's required fields are present.
func (u UpdatePaymentV1Req) Validate(v *validator.Validator) {
	v.Check(len(u.Status) != 0, "status", "must be provided")
}
