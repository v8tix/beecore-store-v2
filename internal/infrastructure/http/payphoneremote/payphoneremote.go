package payphoneremote

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/v8tix/beecore-http/messages/payphone/v1/orders/confirm"
	"github.com/v8tix/kawa"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
)

// Sentinel errors mirroring the ones payphone_payment_strategy_repository.go
// defines in the source repo. GetProviderName/ErrCanonicalOrderRequired/
// ErrTransformOrderFailed/ErrPayPhoneUserLookupFailed have no counterpart
// here: the first is strategy-selection machinery dropped along with
// PayPal (spec assumption #5); the second and third existed only to guard
// the source's *model.CanonicalPaymentRequest / TransformerRegistry
// indirection, both replaced by the plain domain.PaymentRequest value and
// direct calls below; the fourth belonged to GeneratePayPhoneConfig, not
// ported — see this package's doc comment.
var (
	// ErrProductsRequired mirrors ErrProductsRequired: req.Financials must
	// carry at least one product across its store line items.
	ErrProductsRequired = errors.New("at least one product is required for PayPhone payment")

	// ErrOrderTotalInvalid mirrors ErrOrderTotalInvalid.
	ErrOrderTotalInvalid = errors.New("order total must be greater than 0 for PayPhone payment")

	// ErrMissingClientTxID mirrors ErrMissingClientTxID.
	ErrMissingClientTxID = errors.New("client transaction ID is required for PayPhone payment processing")

	// ErrPayPhoneOrderValidation mirrors ErrPayPhoneOrderValidation.
	ErrPayPhoneOrderValidation = errors.New("PayPhone order validation failed")

	// ErrMissingPayPhoneParams mirrors ErrMissingPayPhoneParams.
	ErrMissingPayPhoneParams = errors.New("missing required PayPhone parameters: id or clientTransactionId")

	// ErrInvalidTransactionID mirrors ErrInvalidTransactionID.
	ErrInvalidTransactionID = errors.New("invalid PayPhone transaction ID")

	// ErrPayPhoneConfirmationAPI mirrors ErrPayPhoneConfirmationAPI.
	ErrPayPhoneConfirmationAPI = errors.New("PayPhone confirmation API error")
)

// payPhoneConfirmAPIURL is PayPhone's confirmation endpoint, hit directly
// (not through any beecore-* microservice) — mirrors the literal apiURL in
// PayPhoneRepositoryStrategy.callConfirmAPI
// (payphone_payment_strategy_repository.go:182 in the source repo). A var,
// not a const, solely so this package's own tests can point ConfirmPayment
// at an httptest.Server instead of PayPhone's real gateway; production
// behavior — the URL PayPhone actually receives requests at — is
// unchanged.
var payPhoneConfirmAPIURL = "https://pay.payphonetodoesposible.com/api/button/V2/Confirm"

// PayPhone status values from a confirmation response that indicate the
// transaction was actually approved. Mirrors
// PayPhoneStatusCodeApproved/PayPhoneStatusApproved in the source repo.
const (
	payPhoneStatusCodeApproved = 3
	payPhoneStatusApproved     = "Approved"
)

// defaultCurrency mirrors dto.DefaultCurrency in the source repo.
const defaultCurrency = "USD"

// referenceMaxLength mirrors the source's generateReference local
// constant maxReferenceLength.
const referenceMaxLength = 100

// descriptionMaxLength mirrors the 255-char cap
// PayPhoneTransformer.TransformProduct enforces on a product's
// description.
const descriptionMaxLength = 255

// payPhoneProduct is a single line item as PayPhone's wire order format
// expects it, mirroring model.PayPhoneProduct in the source repo.
type payPhoneProduct struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	Quantity    int     `json:"quantity"`
	Tax         float64 `json:"tax,omitempty"`
	Discount    float64 `json:"discount,omitempty"`
}

// payPhoneOrderRequest is PayPhone's wire order format, mirroring
// model.PayPhoneOrderRequest in the source repo. ProcessPayment never
// actually sends this over HTTP (PayPhone's JS widget does that
// client-side) — it's built and validated here purely so
// PaymentResult.Metadata carries exactly what the source repo audited,
// and so ValidatePayPhoneOrder's checks (see validatePayPhoneOrder below)
// run before the checkout page ever reaches PayPhone's widget.
type payPhoneOrderRequest struct {
	ClientTransactionID string            `json:"clientTransactionId"`
	Products            []payPhoneProduct `json:"products"`
	Amount              float64           `json:"amount"`
	Tax                 float64           `json:"tax"`
	Service             float64           `json:"service"`
	Tip                 float64           `json:"tip"`
	Currency            string            `json:"currency"`
	Reference           string            `json:"reference,omitempty"`
}

// ProcessPayment mirrors PayPhoneRepositoryStrategy.ProcessPayment: it
// validates req, transforms its financial breakdown into PayPhone's wire
// order format (field-for-field, see transformOrder/transformProduct
// below), and returns the confirm/cancel callback URLs. No HTTP call is
// made — PayPhone doesn't redirect; the actual charge happens inline via
// its JS widget hitting ConfirmURL once the shopper completes payment.
func (c *Client) ProcessPayment(_ context.Context, req domain.PaymentRequest) (domain.PaymentResult, error) {
	if !hasAnyProduct(req.Financials) {
		return domain.PaymentResult{}, ErrProductsRequired
	}

	if req.Financials.Total <= 0 {
		return domain.PaymentResult{}, ErrOrderTotalInvalid
	}

	// Transaction ID should be provided from the caller to ensure
	// consistency between PayPhone config generation and payment
	// processing.
	if req.ClientTransactionID == "" {
		return domain.PaymentResult{}, ErrMissingClientTxID
	}

	taxRate := float64(c.cfg.BusinessParameters.Country.Taxes.Percentage) / 100.0
	payphoneOrder := transformOrder(req, taxRate)

	if err := validatePayPhoneOrder(payphoneOrder); err != nil {
		return domain.PaymentResult{}, fmt.Errorf("%w: %v", ErrPayPhoneOrderValidation, err)
	}

	baseURL := c.cfg.Payphone.BaseURL
	confirmURL := fmt.Sprintf("%s/payphone/confirm", baseURL)
	cancelURL := fmt.Sprintf("%s/payphone/cancel", baseURL)

	return domain.PaymentResult{
		PaymentID:   req.ClientTransactionID,
		OrderID:     req.OrderID,
		Status:      "CREATED",
		RedirectURL: "", // PayPhone doesn't redirect, it handles payment inline
		ConfirmURL:  confirmURL,
		CancelURL:   cancelURL,
		Metadata: map[string]any{
			"provider":            "payphone",
			"clientTransactionID": req.ClientTransactionID,
			"payphoneOrder":       payphoneOrder,
		},
	}, nil
}

// ConfirmPayment mirrors PayPhoneRepositoryStrategy.ConfirmPayment: parses
// payphoneID, calls PayPhone's confirmation API directly (see
// payPhoneConfirmAPIURL), and maps its response onto
// domain.PaymentConfirmation field-for-field.
func (c *Client) ConfirmPayment(ctx context.Context, payphoneID, clientTransactionID string) (domain.PaymentConfirmation, error) {
	if payphoneID == "" || clientTransactionID == "" {
		return domain.PaymentConfirmation{}, ErrMissingPayPhoneParams
	}

	id, err := strconv.Atoi(payphoneID)
	if err != nil {
		return domain.PaymentConfirmation{}, fmt.Errorf("%w: %v", ErrInvalidTransactionID, err)
	}

	env, err := c.callConfirmAPI(ctx, id, clientTransactionID)
	if err != nil {
		return domain.PaymentConfirmation{}, fmt.Errorf("%w: %v", ErrPayPhoneConfirmationAPI, err)
	}
	if env.Body == nil {
		return domain.PaymentConfirmation{}, fmt.Errorf("%w: empty response body", ErrPayPhoneConfirmationAPI)
	}

	res := env.Body

	confirmation := domain.PaymentConfirmation{
		TransactionID:     strconv.Itoa(res.TransactionID),
		PaymentID:         res.ClientTransactionID,
		Status:            res.TransactionStatus,
		Amount:            float64(res.Amount) / 100, // Convert from cents
		Currency:          res.Currency,
		AuthorizationCode: res.AuthorizationCode,
		CardType:          res.CardType,
		CardBrand:         res.CardBrand,
		LastDigits:        res.LastDigits,
		Email:             res.Email,
		PhoneNumber:       res.PhoneNumber,
		Document:          res.Document,
		Reference:         res.Reference,
		Date:              res.Date,
	}

	// Check if payment was approved.
	if res.StatusCode != payPhoneStatusCodeApproved || res.TransactionStatus != payPhoneStatusApproved {
		if res.Message != nil {
			confirmation.ErrorMessage = *res.Message
		} else {
			confirmation.ErrorMessage = "Payment was not approved"
		}
	}

	return confirmation, nil
}

// callConfirmAPI makes the actual API call to PayPhone's confirmation
// endpoint. Mirrors PayPhoneRepositoryStrategy.callConfirmAPI exactly,
// including reusing the beecore-http confirm.PayPhoneConfirmReq/Res wire
// types instead of redefining them.
func (c *Client) callConfirmAPI(ctx context.Context, id int, clientTxID string) (*kawa.Envelope[confirm.PayPhoneConfirmRes], error) {
	req := &confirm.PayPhoneConfirmReq{
		ID:         id,
		ClientTxID: clientTxID,
	}

	call := kawa.NewCall[confirm.PayPhoneConfirmReq, confirm.PayPhoneConfirmRes](c.cfg.Web.HTTPClient, kawa.Post, payPhoneConfirmAPIURL).
		WithHeaders(buildPayPhoneHeaders(c.cfg.Payphone.Token)).
		WithDeadline(c.deadline()).
		WithRetryPolicy(c.retryPolicy())

	return call.DoWithRetry(ctx, req)
}

// CancelPayment mirrors PayPhoneRepositoryStrategy.CancelPayment: PayPhone
// handles cancellation automatically if confirmation isn't done within 5
// minutes. Manual cancellation would require PayPhone's separate reverse
// API, never implemented upstream either — so this is a deliberate no-op,
// preserved exactly rather than silently dropped.
func (c *Client) CancelPayment(_ context.Context, _, _ string) error {
	return nil
}

// hasAnyProduct mirrors the source's `len(req.CanonicalOrder.Products) ==
// 0` check against domain.Financials' per-store grouping (the source's
// CanonicalOrder.Products was already a flat list, built ahead of time by
// dto.ToCanonicalProductsFromFinancials from the very same financial
// items this package receives as req.Financials).
func hasAnyProduct(financials domain.Financials) bool {
	for _, item := range financials.Items {
		if len(item.Products) > 0 {
			return true
		}
	}

	return false
}

// transformProduct converts a single basket product into PayPhone's wire
// format. Mirrors PayPhoneTransformer.TransformProduct in the source
// repo's dto/payphone_dto.go, field-for-field:
//
//   - Price falls back to Subtotal/Quantity when UnitPrice (product.Price
//     here) is zero — same fallback the source applies.
//   - Tax is Price*taxRate; the source's per-product TaxRate override
//     branch (`if product.TaxRate > 0`) never triggers here because
//     dto.ToCanonicalProductsFromFinancials — the source's own upstream
//     step — always left CanonicalProduct.TaxRate at its zero value ("Will
//     be set by transformer based on config"), so this port omits the dead
//     branch rather than porting unreachable code.
//   - Description falls back to "Product from {storeName}" when empty,
//     then truncates to 255 chars (252 + "...") — same as the source.
func transformProduct(product domain.BasketProduct, taxRate float64, storeName string) payPhoneProduct {
	price := product.Price
	if price == 0 && product.Quantity > 0 {
		price = product.Subtotal / float64(product.Quantity)
	}

	tax := price * taxRate

	description := product.Description
	if description == "" {
		description = fmt.Sprintf("Product from %s", storeName)
	}
	if len(description) > descriptionMaxLength {
		description = description[:descriptionMaxLength-3] + "..."
	}

	return payPhoneProduct{
		Name:        product.Name,
		Description: description,
		Price:       price,
		Quantity:    product.Quantity,
		Tax:         tax,
		Discount:    0, // Default to no discount, can be enhanced later
	}
}

// transformOrder converts req into PayPhone's wire order format. Mirrors
// PayPhoneTransformer.TransformOrder in the source repo's
// dto/payphone_dto.go:
//
//   - ClientTransactionID is set to req.OrderID, not
//     req.ClientTransactionID — preserving the source's own
//     order.Identity.OrderID usage exactly (see domain.PaymentRequest's
//     doc comment for why the two identifiers differ).
//   - Amount/Tax/Service map onto Financials.Subtotal/TotalTaxesAmount/
//     TotalShippingAmount — the source's Financials.Subtotal/TaxAmount/
//     ShippingCost under different field names for the same values.
//   - Currency falls back to defaultCurrency ("USD") when req.Currency is
//     empty, same net effect as the source's
//     NewPayPhoneTransformer(order.Financials.Currency, ...) constructor
//     call (itself passed the same value being transformed).
func transformOrder(req domain.PaymentRequest, taxRate float64) payPhoneOrderRequest {
	var products []payPhoneProduct
	var names []string

	for _, item := range req.Financials.Items {
		for _, p := range item.Products {
			products = append(products, transformProduct(p, taxRate, item.Store.Name))
			names = append(names, p.Name)
		}
	}

	currency := req.Currency
	if currency == "" {
		currency = defaultCurrency
	}

	return payPhoneOrderRequest{
		ClientTransactionID: req.OrderID,
		Products:            products,
		Amount:              req.Financials.Subtotal,
		Tax:                 req.Financials.TotalTaxesAmount,
		Service:             req.Financials.TotalShippingAmount,
		Tip:                 0, // Default to no tip
		Currency:            currency,
		Reference:           generateReference(req.OrderID, names),
	}
}

// generateReference creates a human-readable reference for the order.
// Mirrors PayPhoneTransformer.generateReference in the source repo
// exactly: fits as many product names as possible within
// referenceMaxLength, appending "and N more items" when they don't all
// fit.
func generateReference(orderID string, productNames []string) string {
	if len(productNames) == 0 {
		return fmt.Sprintf("Order %s", orderID)
	}

	names := make([]string, 0, len(productNames))
	currentLength := 0

	for i, name := range productNames {
		separator := ""
		if i > 0 {
			separator = ", "
		}

		additionalLength := len(separator) + len(name)

		// Check if adding this product would exceed the limit.
		if currentLength+additionalLength > referenceMaxLength-20 { // Reserve 20 chars for potential "and X more" suffix
			remaining := len(productNames) - i
			if remaining > 0 {
				suffix := fmt.Sprintf(" and %d more items", remaining)
				if currentLength+len(suffix) <= referenceMaxLength {
					return strings.Join(names, ", ") + suffix
				}
			}
			break
		}

		names = append(names, name)
		currentLength += additionalLength
	}

	reference := strings.Join(names, ", ")

	// Final safety check - truncate if still too long.
	if len(reference) > referenceMaxLength {
		reference = reference[:referenceMaxLength-3] + "..."
	}

	return reference
}

// validatePayPhoneOrder mirrors dto.ValidatePayPhoneOrder in the source
// repo exactly.
func validatePayPhoneOrder(order payPhoneOrderRequest) error {
	if order.ClientTransactionID == "" {
		return errors.New("client transaction ID is required")
	}

	if len(order.Products) == 0 {
		return errors.New("at least one product is required")
	}

	if order.Amount <= 0 {
		return errors.New("order amount must be greater than 0")
	}

	for i, p := range order.Products {
		if err := validatePayPhoneProduct(p); err != nil {
			return fmt.Errorf("product %d validation failed: %w", i, err)
		}
	}

	return nil
}

// validatePayPhoneProduct mirrors dto.validatePayPhoneProduct in the
// source repo exactly.
func validatePayPhoneProduct(product payPhoneProduct) error {
	if product.Name == "" {
		return errors.New("product name is required")
	}

	if product.Price <= 0 {
		return errors.New("product price must be greater than 0")
	}

	if product.Quantity <= 0 {
		return errors.New("product quantity must be greater than 0")
	}

	return nil
}
