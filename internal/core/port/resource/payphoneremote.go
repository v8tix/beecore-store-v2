package resource

import (
	"context"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
)

//go:generate mockery --name PayPhoneRemote

// PayPhoneRemote is the outbound HTTP boundary for the payment slice,
// ported from
// internal/business/core/repository/payphone_payment_strategy_repository.go
// in the source repo (PayPhoneRepositoryStrategy). PayPal is deprecated
// and not ported (spec assumption #5) — with a single provider, the
// source's PaymentStrategy interface's GetProviderName (provider
// selection) has nothing left to select between and is dropped; the other
// three methods are carried over 1:1, including PayPhoneOrderRequest's
// canonical→wire DTO mapping logic (PayPhoneTransformer.TransformProduct/
// TransformOrder in the source's dto/payphone_dto.go), inlined directly
// into this adapter's implementation instead of routed through the
// source's generic TransformerRegistry (dropped entirely, see spec
// assumption #5).
type PayPhoneRemote interface {
	// ProcessPayment validates req and transforms its financial breakdown
	// into PayPhone's wire order format (for audit/logging via
	// PaymentResult.Metadata — the actual charge happens client-side via
	// PayPhone's JS widget, this call makes no external request), then
	// returns the confirm/cancel callback URLs a checkout page embeds for
	// that widget. Mirrors PayPhoneRepositoryStrategy.ProcessPayment.
	ProcessPayment(ctx context.Context, req domain.PaymentRequest) (domain.PaymentResult, error)

	// ConfirmPayment confirms a PayPhone transaction: a direct HTTP call
	// to PayPhone's own gateway (pay.payphonetodoesposible.com/api/button/
	// V2/Confirm), not one of our own beecore-* microservices. payphoneID
	// is PayPhone's numeric transaction ID (the "id" query parameter on
	// PayPhone's confirm callback, still a string here — parsed once
	// inside the adapter, same as the source repository did with its own
	// strconv.Atoi). Mirrors PayPhoneRepositoryStrategy.ConfirmPayment;
	// preserves its exact request/response shape and URL — see this
	// package's doc comment.
	ConfirmPayment(ctx context.Context, payphoneID, clientTransactionID string) (domain.PaymentConfirmation, error)

	// CancelPayment mirrors PayPhoneRepositoryStrategy.CancelPayment: a
	// deliberate no-op. PayPhone auto-cancels an unconfirmed transaction
	// if the shopper doesn't confirm within 5 minutes; manual
	// cancellation would require PayPhone's separate reverse API, never
	// implemented upstream either.
	CancelPayment(ctx context.Context, payphoneID, clientTransactionID string) error
}
