// Package payment is the payment use-case, ported from
// internal/business/core/repository/payphone_payment_strategy_repository.go
// in the source repo at the service layer (see port/service.Payment's doc
// comment for the full mapping). PayPal is deprecated and not ported (spec
// assumption #5): unlike the source's PaymentStrategy interface — resolved
// at runtime to one of two concrete strategies via
// cfg.BusinessParameters.Country.PaymentStrategy — ProcessPayment/
// ConfirmPayment/CancelPayment are mostly thin pass-throughs straight to
// resource.PayPhoneRemote, the single implementation, plus
// resource.PaymentRecordRemote's audit/idempotency bookkeeping (see that
// interface's doc comment for why it's a separate dependency from the
// PayPhone gateway call itself). PayPhone's gateway is authenticated with
// its own static Payphone.Token (config, held by the
// resource.PayPhoneRemote adapter), never our downstream microservices'
// per-request admin/user token — AuthRemote/PaymentRecordRemote calls use
// an admin token instead, the same "reuse GetAdminToken from AuthRemote"
// pattern every other core/*.Service in this repo already uses.
package payment

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/core/port/resource"
	"github.com/v8tix/beecore-store-v2/internal/core/port/service"
	"github.com/v8tix/beecore-store-v2/internal/token"
)

// paymentPlatform is the value recorded on every payment record — PayPal
// is deprecated and not ported (spec assumption #5), so this is the only
// provider going forward. Mirrors domain.Payment's own doc comment.
const paymentPlatform = "payphone"

// Payment record status values, mirroring the
// repository.PaymentPending/PaymentConfirmed/PaymentFailed constants in
// the source repo's internal/business/core/repository/payments_repository.go.
const (
	paymentStatusPending   = "PENDING"
	paymentStatusConfirmed = "CONFIRMED"
	paymentStatusFailed    = "FAILED"
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

	// PaymentRecordRemote records (and later updates) beecore-payments'
	// own audit/idempotency record of each payment attempt — see that
	// interface's doc comment. Used by ProcessPayment (create, PENDING)
	// and ConfirmPayment (find + validate, then update to
	// CONFIRMED/FAILED).
	PaymentRecordRemote resource.PaymentRecordRemote
}

type paymentService struct {
	deps Dependencies
}

var _ service.Payment = (*paymentService)(nil)

func NewPaymentService(d Dependencies) *paymentService {
	return &paymentService{deps: d}
}

// ProcessPayment mirrors Repository.ProcessPayment (via the
// PaymentStrategy interface) exposed at the service layer, plus — unlike
// the source's pass-through — the source handler's own CreatePayment call
// (cmd/web/payphone_handlers.go's PayPhoneCreatePayment), ported down to
// this layer since PaymentHandler.ProcessPayment never had it either (see
// this package's and resource.PaymentRecordRemote's doc comments). The
// payment record's id is PayPhoneRemote.ProcessPayment's resulting
// PaymentID (== req.ClientTransactionID, see domain.PaymentRequest's doc
// comment) so ConfirmPayment can look it up by the same id PayPhone later
// echoes back.
func (s *paymentService) ProcessPayment(ctx context.Context, req domain.PaymentRequest) (domain.PaymentResult, error) {
	result, err := s.deps.PayPhoneRemote.ProcessPayment(ctx, req)
	if err != nil {
		return domain.PaymentResult{}, err
	}

	at, err := s.deps.AuthRemote.GetAdminToken(ctx)
	if err != nil {
		return domain.PaymentResult{}, err
	}

	requestJSON, err := json.Marshal(req)
	if err != nil {
		return domain.PaymentResult{}, fmt.Errorf("marshal payment request: %w", err)
	}

	if err := s.deps.PaymentRecordRemote.CreatePayment(
		ctx, result.PaymentID, req.UserID, paymentPlatform, paymentStatusPending, string(requestJSON), "", req.Financials.Total, at,
	); err != nil {
		return domain.PaymentResult{}, err
	}

	return result, nil
}

// ConfirmPayment mirrors the source repo's PayPhoneConfirm handler
// (cmd/web/payphone_handlers.go) at the service layer — see
// port/service.Payment's doc comment for the full behavior. Validates the
// payment record ProcessPayment created is still PENDING (rejecting a
// retried/replayed callback outright, mirroring the source's
// ErrPaymentStatusValidationFailed check, before even calling PayPhone
// again), confirms against PayPhone, then updates that record to
// CONFIRMED or FAILED depending on the result — mirroring the source's own
// "update the record either way, then fail the request if not approved"
// order so a declined card still leaves an honest FAILED record behind.
// The order/basket sequence only runs once PayPhone's confirmation
// reports the transaction approved. A non-approved result
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
	at, err := s.deps.AuthRemote.GetAdminToken(ctx)
	if err != nil {
		return domain.PaymentConfirmation{}, "", err
	}

	existing, err := s.deps.PaymentRecordRemote.FindPayment(ctx, paymentID, at)
	if err != nil {
		return domain.PaymentConfirmation{}, "", err
	}
	if existing.Status != paymentStatusPending {
		return domain.PaymentConfirmation{}, "", domain.ErrPaymentNotPending
	}

	confirmation, err := s.deps.PayPhoneRemote.ConfirmPayment(ctx, payphoneID, clientTransactionID)
	if err != nil {
		return domain.PaymentConfirmation{}, "", err
	}

	newStatus := paymentStatusConfirmed
	if confirmation.ErrorMessage != "" {
		newStatus = paymentStatusFailed
	}

	responseJSON, err := json.Marshal(confirmation)
	if err != nil {
		return domain.PaymentConfirmation{}, "", fmt.Errorf("marshal payment confirmation: %w", err)
	}

	if err := s.deps.PaymentRecordRemote.UpdatePayment(
		ctx, paymentID, newStatus, existing.Request, string(responseJSON), at,
	); err != nil {
		return domain.PaymentConfirmation{}, "", err
	}

	if confirmation.ErrorMessage != "" {
		return confirmation, "", nil
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
