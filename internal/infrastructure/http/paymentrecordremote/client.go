// Package paymentrecordremote is the kawa-based HTTP client implementing
// resource.PaymentRecordRemote, ported from
// internal/business/core/repository/payments_repository.go in the source
// repo (see port/resource.PaymentRecordRemote's doc comment for the full
// mapping).
package paymentrecordremote

import (
	"time"

	"github.com/v8tix/beecore-eda/v2/config"
	"github.com/v8tix/kawa"

	"github.com/v8tix/beecore-store-v2/internal/core/port/resource"
	"github.com/v8tix/beecore-store-v2/internal/infrastructure/http/httpshared"
)

// Client is the outbound HTTP boundary for beecore-payments' payment-record
// API. It holds *config.Cfg directly (same as every other remote adapter
// in this repo) — Integration.V1.PaymentsURL, the shared *http.Client, and
// the retry/deadline profile it needs all live there already.
type Client struct {
	cfg *config.Cfg
}

var _ resource.PaymentRecordRemote = (*Client)(nil)

func NewClient(cfg *config.Cfg) *Client {
	return &Client{cfg: cfg}
}

// deadline and retryPolicy mirror every other remote adapter in this repo:
// every call here is a single-record create/read/update, so they use the
// config-tunable "fast" HTTP client profile.
func (c *Client) deadline() time.Duration {
	return httpshared.Deadline(c.cfg)
}

func (c *Client) retryPolicy() kawa.RetryPolicy {
	return httpshared.RetryPolicy(c.cfg)
}
