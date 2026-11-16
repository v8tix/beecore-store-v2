package paymentrecordremote

import (
	"context"
	"fmt"

	payments "github.com/v8tix/beecore-http/messages/payments/v1"
	"github.com/v8tix/kawa"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/infrastructure/http/httpshared"
)

// CreatePayment mirrors BaseRepositoryImpl.CreatePayment (POST payments).
func (c *Client) CreatePayment(
	ctx context.Context, id, userID, platform, status, request, response string, amount float64, token string,
) error {
	url := fmt.Sprintf("%s/%s", c.cfg.Integration.V1.PaymentsURL, "payments")

	req := payments.CreatePaymentV1Req{
		PaymentID: id,
		UserID:    userID,
		Amount:    amount,
		Platform:  platform,
		Status:    status,
		Request:   request,
		Response:  response,
	}

	call := kawa.NewCall[payments.CreatePaymentV1Req, payments.CreatePaymentEnvV1Res](c.cfg.Web.HTTPClient, kawa.Post, url).
		WithHeaders(httpshared.BuildAuthHeader(token)).
		WithDeadline(c.deadline()).
		WithRetryPolicy(c.retryPolicy())

	if _, err := call.DoWithRetry(ctx, &req); err != nil {
		return httpshared.TranslateHTTPError(err)
	}

	return nil
}

// FindPayment mirrors BaseRepositoryImpl.FindPayment (GET payments/{id}).
func (c *Client) FindPayment(ctx context.Context, id, token string) (domain.Payment, error) {
	url := fmt.Sprintf("%s/%s/%s", c.cfg.Integration.V1.PaymentsURL, "payments", id)

	call := kawa.NewCall[kawa.NoReq, payments.PaymentEnvV1Res](c.cfg.Web.HTTPClient, kawa.Get, url).
		WithHeaders(httpshared.BuildAuthHeader(token)).
		WithDeadline(c.deadline()).
		WithRetryPolicy(c.retryPolicy())

	env, err := call.DoWithRetry(ctx, nil)
	if err != nil {
		return domain.Payment{}, httpshared.TranslateHTTPError(err)
	}

	if env.Body == nil {
		return domain.Payment{}, nil
	}

	p := env.Body.Payment

	return domain.Payment{
		ID:        p.PaymentID,
		UserID:    p.UserID,
		Amount:    p.Amount,
		Platform:  p.Platform,
		Status:    p.Status,
		Request:   p.Request,
		Response:  p.Response,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}, nil
}

// UpdatePayment mirrors BaseRepositoryImpl.UpdatePayment (PUT payments/{id}).
func (c *Client) UpdatePayment(ctx context.Context, id, status, request, response, token string) error {
	url := fmt.Sprintf("%s/%s/%s", c.cfg.Integration.V1.PaymentsURL, "payments", id)

	req := payments.UpdatePaymentV1Req{
		Status:   status,
		Request:  request,
		Response: response,
	}

	call := kawa.NewCall[payments.UpdatePaymentV1Req, kawa.NoRes](c.cfg.Web.HTTPClient, kawa.Put, url).
		WithHeaders(httpshared.BuildAuthHeader(token)).
		WithDeadline(c.deadline()).
		WithRetryPolicy(c.retryPolicy())

	if _, err := call.DoWithRetry(ctx, &req); err != nil {
		return httpshared.TranslateHTTPError(err)
	}

	return nil
}
