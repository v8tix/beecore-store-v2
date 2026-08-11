package service

import (
	"context"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
)

//go:generate mockery --name Payment

// Payment is the payment use-case, ported from
// internal/business/core/repository/payphone_payment_strategy_repository.go
// in the source repo at the service layer. PayPal is deprecated and not
// ported (spec assumption #5): unlike the source's PaymentStrategy
// interface (implemented separately by PayPhoneRepositoryStrategy and
// PayPalRepositoryStrategy, selected at runtime via
// cfg.BusinessParameters.Country.PaymentStrategy), there is only one
// implementation of this interface — core/payment.Service — and it calls
// resource.PayPhoneRemote directly, no strategy/registry indirection.
type Payment interface {
	// ProcessPayment mirrors Repository.ProcessPayment (via the
	// PaymentStrategy interface) at the service layer — validates and
	// transforms a checked-out basket into PayPhone's wire order format,
	// returning the confirm/cancel callback URLs a checkout page embeds
	// for PayPhone's JS widget.
	ProcessPayment(ctx context.Context, req domain.PaymentRequest) (domain.PaymentResult, error)

	// ConfirmPayment mirrors Repository.ConfirmPayment at the service
	// layer — confirms a PayPhone transaction against PayPhone's own
	// gateway (see resource.PayPhoneRemote's doc comment).
	ConfirmPayment(ctx context.Context, payphoneID, clientTransactionID string) (domain.PaymentConfirmation, error)

	// CancelPayment mirrors Repository.CancelPayment at the service
	// layer — a deliberate no-op, see resource.PayPhoneRemote's doc
	// comment.
	CancelPayment(ctx context.Context, payphoneID, clientTransactionID string) error
}
