// Package payment is the payment use-case, ported from
// internal/business/core/repository/payphone_payment_strategy_repository.go
// in the source repo at the service layer (see port/service.Payment's doc
// comment for the full mapping). PayPal is deprecated and not ported (spec
// assumption #5): unlike the source's PaymentStrategy interface — resolved
// at runtime to one of two concrete strategies via
// cfg.BusinessParameters.Country.PaymentStrategy — every method here is a
// thin pass-through straight to resource.PayPhoneRemote, the single
// implementation. Unlike core/basket.Service, core/order.Service and
// core/product.Service, this service resolves no admin token: PayPhone's
// gateway is authenticated with its own static Payphone.Token (config,
// held by the resource.PayPhoneRemote adapter), never our downstream
// microservices' per-request admin/user token.
package payment

import (
	"context"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/core/port/resource"
	"github.com/v8tix/beecore-store-v2/internal/core/port/service"
)

// Dependencies are paymentService's collaborators, all resolved once at
// composition-root time rather than read from *config.Cfg here — core
// stays free of any infra/config import.
type Dependencies struct {
	PayPhoneRemote resource.PayPhoneRemote
}

type paymentService struct {
	deps Dependencies
}

var _ service.Payment = (*paymentService)(nil)

func NewPaymentService(d Dependencies) *paymentService {
	return &paymentService{deps: d}
}

// ProcessPayment mirrors Repository.ProcessPayment (via the
// PaymentStrategy interface) exposed at the service layer.
func (s *paymentService) ProcessPayment(ctx context.Context, req domain.PaymentRequest) (domain.PaymentResult, error) {
	return s.deps.PayPhoneRemote.ProcessPayment(ctx, req)
}

// ConfirmPayment mirrors Repository.ConfirmPayment exposed at the service
// layer.
func (s *paymentService) ConfirmPayment(ctx context.Context, payphoneID, clientTransactionID string) (domain.PaymentConfirmation, error) {
	return s.deps.PayPhoneRemote.ConfirmPayment(ctx, payphoneID, clientTransactionID)
}

// CancelPayment mirrors Repository.CancelPayment exposed at the service
// layer.
func (s *paymentService) CancelPayment(ctx context.Context, payphoneID, clientTransactionID string) error {
	return s.deps.PayPhoneRemote.CancelPayment(ctx, payphoneID, clientTransactionID)
}
