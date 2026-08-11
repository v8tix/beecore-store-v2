// Package payphoneremote is the kawa-based HTTP client implementing
// resource.PayPhoneRemote, ported from
// internal/business/core/repository/payphone_payment_strategy_repository.go
// in the source repo (see port/resource.PayPhoneRemote's doc comment for
// the full mapping). PayPal is deprecated and not ported (spec assumption
// #5) — there is no payphoneremote/paypalremote split, no strategy
// interface, no generic transformer registry: PayPhone's canonical→wire
// DTO mapping logic (source's dto/payphone_dto.go
// PayPhoneTransformer.TransformProduct/TransformOrder) is inlined
// directly below, field-for-field, instead of routed through the source's
// TransformerRegistry.
package payphoneremote

import (
	"fmt"
	"time"

	"github.com/v8tix/beecore-eda/config"
	"github.com/v8tix/kawa"

	"github.com/v8tix/beecore-store-v2/internal/core/port/resource"
)

// Client is the outbound HTTP boundary for the payment vertical slice. It
// holds *config.Cfg directly (same as source repo's BaseRepositoryImpl
// struct) — the Payphone.Token/BaseURL and BusinessParameters.Country.Taxes
// config it needs, and the shared *http.Client, all live there already.
//
// No repo-wide httpshared package exists yet, so
// deadline/retryPolicy/buildAuthHeader are duplicated here rather than
// factored out prematurely, matching orderremote's, authremote's,
// addressremote's, userremote's, productremote's and basketremote's
// existing approach.
type Client struct {
	cfg *config.Cfg
}

var _ resource.PayPhoneRemote = (*Client)(nil)

func NewClient(cfg *config.Cfg) *Client {
	return &Client{cfg: cfg}
}

// deadline and retryPolicy mirror BaseRepositoryImpl.deadline/retryPolicy
// in the source repo: ConfirmPayment is the one method here that makes an
// actual HTTP call, and — like every non-overridden BaseRepositoryImpl
// method — uses the config-tunable "fast" HTTP client profile (unlike
// orderremote's GetOrdersByUserID, PayPhoneRepositoryStrategy.
// callConfirmAPI in the source repo does not override it).
func (c *Client) deadline() time.Duration {
	return c.cfg.HTTPClient.GetFast().Timeout.Duration()
}

func (c *Client) retryPolicy() kawa.RetryPolicy {
	profile := c.cfg.HTTPClient.GetFast()
	return kawa.NewExponentialRetryPolicy(profile.MaxRetries, profile.RetryInterval.Duration())
}

// buildPayPhoneHeaders mirrors the headers map built inline by
// PayPhoneRepositoryStrategy.callConfirmAPI in the source repo: PayPhone's
// gateway is authenticated with the app's static Payphone.Token (a
// bearer credential issued by PayPhone itself), never our own downstream
// microservices' per-request admin/user token — unlike every other
// adapter in this repo, ConfirmPayment takes no token parameter.
func buildPayPhoneHeaders(token string) map[string]string {
	return map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
		"Content-Type":  "application/json",
	}
}
