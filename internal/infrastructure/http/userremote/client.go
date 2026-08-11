// Package userremote is the kawa-based HTTP client implementing
// resource.UserRemote, ported 1:1 from the UpdateUser method of
// internal/business/core/repository/user_repository.go in the source repo
// (see port/resource.UserRemote's doc comment for why it's the only
// method that landed here).
package userremote

import (
	"time"

	"github.com/v8tix/beecore-eda/config"
	"github.com/v8tix/kawa"

	"github.com/v8tix/beecore-store-v2/internal/core/port/resource"
	"github.com/v8tix/beecore-store-v2/internal/infrastructure/http/httpshared"
)

// Client is the outbound HTTP boundary for the user profile vertical
// slice. It holds *config.Cfg directly (same as source repo's
// BaseRepositoryImpl struct) — the URLs and shared *http.Client it needs
// all live there already.
type Client struct {
	cfg *config.Cfg
}

var _ resource.UserRemote = (*Client)(nil)

func NewClient(cfg *config.Cfg) *Client {
	return &Client{cfg: cfg}
}

// deadline and retryPolicy mirror BaseRepositoryImpl.deadline/retryPolicy
// in the source repo: every call here is a single-record lookup, so they
// use the config-tunable "fast" HTTP client profile.
func (c *Client) deadline() time.Duration {
	return httpshared.Deadline(c.cfg)
}

func (c *Client) retryPolicy() kawa.RetryPolicy {
	return httpshared.RetryPolicy(c.cfg)
}
