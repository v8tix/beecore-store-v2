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

// PaymentRequest is the input to service.Payment.ProcessPayment /
// resource.PayPhoneRemote.ProcessPayment — a checked-out basket ready to be
// validated and transformed into PayPhone's wire order format. Mirrors
// model.CanonicalPaymentRequest in the source repo, narrowed to the fields
// PayPhoneRepositoryStrategy.ProcessPayment actually reads:
//
//   - OrderID/ClientTransactionID carry the two distinct identifiers the
//     source keeps side by side: OrderID is a fresh identity minted once
//     per checkout attempt (source: uuid.New(), in
//     dto.ToCanonicalPaymentRequest) and becomes both PaymentResult.OrderID
//     and — preserved exactly, including this apparent inconsistency in the
//     source — the ClientTransactionID field PayPhoneRepositoryStrategy
//     embeds in the transformed wire order itself
//     (PayPhoneTransformer.TransformOrder uses order.Identity.OrderID, not
//     the request's own ClientTransactionID). ClientTransactionID is the
//     value generated once by the caller for PayPhone widget/session
//     consistency (source: BaseRepositoryImpl.GenerateTransactionID) and
//     becomes PaymentResult.PaymentID.
//   - Financials is the basket's cost breakdown, as returned by
//     core/basket.Service.ComputeFinancials — replaces the source's
//     CanonicalOrder.Products (flattened from the same financial items via
//     dto.ToCanonicalProductsFromFinancials) and CanonicalOrder.Financials.
//   - Currency is the one value domain.Financials doesn't itself carry (see
//     its doc comment) — mirrors the
//     app.Cfg.BusinessParameters.Country.Currency argument the source
//     passes into dto.ToCanonicalPaymentRequest.
type PaymentRequest struct {
	OrderID             string
	ClientTransactionID string
	Currency            string
	Financials          Financials
}
