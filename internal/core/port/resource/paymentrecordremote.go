package resource

import (
	"context"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
)

//go:generate mockery --name PaymentRecordRemote

// PaymentRecordRemote is the outbound HTTP boundary for beecore-payments'
// own audit/idempotency record of a payment attempt — entirely separate
// from the PayPhone gateway call itself (resource.PayPhoneRemote) and from
// the order/basket orchestration in core/payment.Service.ConfirmPayment.
// Mirrors the source repo's BaseRepositoryImpl.CreatePayment/FindPayment/
// UpdatePayment (internal/business/core/repository/payments_repository.go),
// which this slice was ported without — see core/payment.Service's doc
// comment for why.
type PaymentRecordRemote interface {
	// CreatePayment records a payment attempt as PENDING, before the
	// shopper submits their card to PayPhone. Mirrors
	// BaseRepositoryImpl.CreatePayment (POST payments).
	CreatePayment(ctx context.Context, id, userID, platform, status, request, response string, amount float64, token string) error

	// FindPayment looks up a payment record by id. Mirrors
	// BaseRepositoryImpl.FindPayment (GET payments/{id}).
	FindPayment(ctx context.Context, id, token string) (domain.Payment, error)

	// UpdatePayment updates a payment record's status and request/response
	// payloads — used once PayPhone confirms (or rejects) the attempt.
	// Mirrors BaseRepositoryImpl.UpdatePayment (PUT payments/{id}).
	UpdatePayment(ctx context.Context, id, status, request, response, token string) error
}
