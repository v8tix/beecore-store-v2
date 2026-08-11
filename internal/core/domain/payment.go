package domain

import "time"

// Payment represents a payment record, as exchanged with the downstream
// beecore-payments service (see
// internal/business/core/repository/payments_repository.go in the source
// repo, payments.PaymentV1Res). PayPal is deprecated and not ported (spec
// assumption #5) — Platform is expected to be "payphone" going forward,
// but the field itself is carried over unchanged since the downstream
// service still defines it as a free-form string.
type Payment struct {
	ID        string
	UserID    string
	Amount    float64
	Platform  string
	Status    string
	Request   string
	Response  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// PaymentConfirmation is the result of confirming a payment with the
// payment provider (PayPhone only — PayPal's strategy is not ported, see
// spec assumption #5), as returned by
// internal/business/core/repository/payphone_payment_strategy_repository.go
// in the source repo (model.PaymentConfirmation).
type PaymentConfirmation struct {
	TransactionID     string
	PaymentID         string
	Status            string
	Amount            float64
	Currency          string
	AuthorizationCode string
	CardType          string
	CardBrand         string
	LastDigits        string
	Email             string
	PhoneNumber       string
	Document          string
	Reference         string
	Date              string
	ErrorMessage      string
}

// PaymentCancellation is the result of cancelling a payment with the
// payment provider (model.PaymentCancellationResponse in the source
// repo).
type PaymentCancellation struct {
	Status     string
	CanceledAt string
	Method     string
}

// PaymentResult is the outcome of initiating a payment with the provider —
// where to redirect the shopper, and the confirm/cancel callback URLs the
// provider will hit (model.PaymentResponse in the source repo).
type PaymentResult struct {
	PaymentID    string
	OrderID      string
	Status       string
	RedirectURL  string
	ConfirmURL   string
	CancelURL    string
	ErrorMessage string
	Metadata     map[string]any
}
