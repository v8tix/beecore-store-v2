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

	// OrderRemote and BasketRemote back ConfirmPayment's post-approval
	// order-creation/basket-checkout/basket-recreate sequence (source
	// repo: cmd/web/payphone_handlers.go's PayPhoneConfirm handler). Same
	// "services depend on resources directly, not on each other's
	// service" rule core/auth.Login already applies to
	// resource.AddressRemote — see resource.OrderRemote's and
	// resource.BasketRemote's own doc comments for why CreateOrder/
	// CheckoutBasket were deliberately left unmirrored at the
	// core/order.Service and core/basket.Service layers until this
	// Payment-slice landed.
	OrderRemote  resource.OrderRemote
	BasketRemote resource.BasketRemote
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

// ConfirmPayment mirrors the source repo's PayPhoneConfirm handler
// (cmd/web/payphone_handlers.go) at the service layer — see
// port/service.Payment's doc comment for the full behavior. Confirms
// against PayPhone first; the order/basket sequence only runs once that
// confirmation reports the transaction approved. A non-approved result
// (confirmation.ErrorMessage set — PayPhone reports this as ordinary
// response data, not a Go error, see resource.PayPhoneRemote's doc
// comment) returns with newBasketID empty and no order/basket side
// effects, mirroring the source handler's own confirmation.Status !=
// PayPhoneStatusApproved short circuit: it deliberately leaves the basket
// untouched so the shopper can retry the same basket instead of losing
// their cart on a declined card.
func (s *paymentService) ConfirmPayment(
	ctx context.Context,
	payphoneID, clientTransactionID, userID, basketID, paymentID, shippingAddressID string,
) (domain.PaymentConfirmation, string, error) {
	confirmation, err := s.deps.PayPhoneRemote.ConfirmPayment(ctx, payphoneID, clientTransactionID)
	if err != nil {
		return domain.PaymentConfirmation{}, "", err
	}
	if confirmation.ErrorMessage != "" {
		return confirmation, "", nil
	}

	at, err := s.deps.AuthRemote.GetAdminToken(ctx)
	if err != nil {
		return domain.PaymentConfirmation{}, "", err
	}

	financials, err := s.deps.BasketRemote.ComputeFinancials(ctx, basketID, at)
	if err != nil {
		return domain.PaymentConfirmation{}, "", err
	}

	// One order per store — mirrors the source repo's
	// dto.ToCreateOrdersV1Req: each financial line item groups the
	// basket's products by the store that sells them.
	for _, storeFinancial := range financials.Items {
		_, err := s.deps.OrderRemote.CreateOrder(ctx, domain.CreateOrderRequest{
			UserID:            userID,
			PaymentID:         paymentID,
			BasketID:          basketID,
			ShippingAddressID: shippingAddressID,
			FinancialItem:     storeFinancial,
		}, at)
		if err != nil {
			return domain.PaymentConfirmation{}, "", err
		}
	}

	if err := s.deps.BasketRemote.CheckoutBasket(ctx, basketID, paymentID, shippingAddressID, at); err != nil {
		return domain.PaymentConfirmation{}, "", err
	}

	newBasketID, err := s.deps.BasketRemote.CreateBasket(ctx, userID, at)
	if err != nil {
		return domain.PaymentConfirmation{}, "", err
	}

	return confirmation, newBasketID, nil
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
