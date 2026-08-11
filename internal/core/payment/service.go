// Package payment is the payment use-case, ported from
// internal/business/core/repository/payphone_payment_strategy_repository.go
// in the source repo at the service layer (see port/service.Payment's doc
// comment for the full mapping). PayPal is deprecated and not ported (spec
// assumption #5): unlike the source's PaymentStrategy interface — resolved
// at runtime to one of two concrete strategies via
// cfg.BusinessParameters.Country.PaymentStrategy — every method here is a
// thin pass-through straight to resource.PayPhoneRemote, the single
// implementation. Unlike core/basket.Service, core/order.Service and
// core/product.Service, ProcessPayment/ConfirmPayment/CancelPayment resolve
// no admin token: PayPhone's gateway is authenticated with its own static
// Payphone.Token (config, held by the resource.PayPhoneRemote adapter),
// never our downstream microservices' per-request admin/user token.
// GeneratePayPhoneConfig/GeneratePayPhoneConfigWithTransactionID are the
// exception (plan Task 19, resolving the deferral resource.UserRemote's and
// port/resource.PayPhoneRemote's earlier doc comments described): they need
// an admin token to resolve the shopper's own contact details via
// resource.AuthRemote.FindUserByID, so this service also depends on
// resource.AuthRemote for that one purpose — the same "reuse GetAdminToken
// from AuthRemote" pattern every other core/*.Service in this repo already
// uses, not a new one invented here.
package payment

import (
	"context"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/core/port/resource"
	"github.com/v8tix/beecore-store-v2/internal/core/port/service"
	"github.com/v8tix/beecore-store-v2/internal/token"
)

// Dependencies are paymentService's collaborators, all resolved once at
// composition-root time rather than read from *config.Cfg here — core
// stays free of any infra/config import.
type Dependencies struct {
	PayPhoneRemote resource.PayPhoneRemote

	// AuthRemote is used only for its GetAdminToken and FindUserByID
	// methods, by GeneratePayPhoneConfig/GeneratePayPhoneConfigWithTransactionID
	// — see this package's doc comment.
	AuthRemote resource.AuthRemote
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

// GeneratePayPhoneConfig mirrors Repository.GeneratePayPhoneConfig exposed
// at the service layer: mints a fresh client transaction ID via
// internal/token.New() when req.ClientTransactionID is empty (mirrors the
// source's BaseRepositoryImpl.GenerateTransactionID fallback, same
// generator this repo's PaymentHandler.ProcessPayment already uses for the
// same purpose), then delegates to GeneratePayPhoneConfigWithTransactionID.
func (s *paymentService) GeneratePayPhoneConfig(ctx context.Context, req domain.PaymentRequest) (map[string]any, error) {
	clientTransactionID := req.ClientTransactionID
	if clientTransactionID == "" {
		id, err := token.New()
		if err != nil {
			return nil, err
		}
		clientTransactionID = id
	}

	return s.GeneratePayPhoneConfigWithTransactionID(ctx, req, clientTransactionID)
}

// GeneratePayPhoneConfigWithTransactionID mirrors
// Repository.GeneratePayPhoneConfigWithTransactionID exposed at the service
// layer: resolves req.UserID via resource.AuthRemote (admin token +
// FindUserByID — see this package's doc comment), then delegates the
// config assembly itself to resource.PayPhoneRemote.GeneratePayPhoneConfig.
func (s *paymentService) GeneratePayPhoneConfigWithTransactionID(ctx context.Context, req domain.PaymentRequest, clientTransactionID string) (map[string]any, error) {
	at, err := s.deps.AuthRemote.GetAdminToken(ctx)
	if err != nil {
		return nil, err
	}

	user, err := s.deps.AuthRemote.FindUserByID(ctx, req.UserID, at)
	if err != nil {
		return nil, err
	}

	return s.deps.PayPhoneRemote.GeneratePayPhoneConfig(req, clientTransactionID, user), nil
}
