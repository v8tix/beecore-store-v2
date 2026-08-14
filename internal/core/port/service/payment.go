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

	// ConfirmPayment mirrors the source repo's PayPhoneConfirm handler at
	// the service layer (cmd/web/payphone_handlers.go): confirms a
	// PayPhone transaction against PayPhone's own gateway (see
	// resource.PayPhoneRemote's doc comment), and — only once PayPhone
	// reports the transaction approved — creates one Order per store from
	// basketID's financial breakdown, checks basketID out against
	// paymentID/shippingAddressID, and opens a fresh basket for userID so
	// the shopper's cart is empty for their next visit. newBasketID is
	// that fresh basket's ID, always non-empty when err is nil; the
	// caller must persist it as the session's new basket ID (and reset
	// its cached item count to 0) or the shopper's cart will keep showing
	// the just-paid-for basket indefinitely.
	ConfirmPayment(
		ctx context.Context,
		payphoneID, clientTransactionID, userID, basketID, paymentID, shippingAddressID string,
	) (confirmation domain.PaymentConfirmation, newBasketID string, err error)

	// CancelPayment mirrors Repository.CancelPayment at the service
	// layer — a deliberate no-op, see resource.PayPhoneRemote's doc
	// comment.
	CancelPayment(ctx context.Context, payphoneID, clientTransactionID string) error

	// GeneratePayPhoneConfig resolves req.UserID's contact details (via
	// resource.AuthRemote.FindUserByID) and builds the checkout page's
	// PayPhone JS-widget configuration, minting a fresh client transaction
	// ID first if req.ClientTransactionID is empty. Mirrors
	// PayPhoneRepositoryStrategy.GeneratePayPhoneConfig at the service
	// layer (plan Task 19 — see resource.AuthRemote's and
	// resource.PayPhoneRemote's doc comments for why the user lookup moved
	// here).
	GeneratePayPhoneConfig(ctx context.Context, req domain.PaymentRequest) (map[string]any, error)

	// GeneratePayPhoneConfigWithTransactionID is GeneratePayPhoneConfig
	// with an explicit, caller-chosen client transaction ID instead of
	// minting one — used when the checkout page has already persisted an
	// ID (in the browser session) it needs the widget config to stay
	// consistent with. Mirrors
	// PayPhoneRepositoryStrategy.GeneratePayPhoneConfigWithTransactionID at
	// the service layer.
	GeneratePayPhoneConfigWithTransactionID(ctx context.Context, req domain.PaymentRequest, clientTransactionID string) (map[string]any, error)
}
