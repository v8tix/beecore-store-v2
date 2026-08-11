// Package userremote is the kawa-based HTTP client implementing
// resource.UserRemote, ported 1:1 from the UpdateUser method of
// internal/business/core/repository/user_repository.go in the source repo
// (see port/resource.UserRemote's doc comment for why it's the only
// method that landed here).
package userremote

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/v8tix/beecore-eda/config"
	"github.com/v8tix/kawa"

	"github.com/v8tix/beecore-store-v2/internal/core/port/resource"
)

// Client is the outbound HTTP boundary for the user profile vertical
// slice. It holds *config.Cfg directly (same as source repo's
// BaseRepositoryImpl struct) — the URLs and shared *http.Client it needs
// all live there already.
//
// A repo-wide httpshared package doesn't exist yet (this is the second
// adapter package ported after authremote, which took the same approach)
// — deadline/retryPolicy/buildAuthHeader/decodeHTTPError are duplicated
// here rather than factored out prematurely; a dedup pass is future work,
// not part of this 1:1 port.
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
	return c.cfg.HTTPClient.GetFast().Timeout.Duration()
}

func (c *Client) retryPolicy() kawa.RetryPolicy {
	profile := c.cfg.HTTPClient.GetFast()
	return kawa.NewExponentialRetryPolicy(profile.MaxRetries, profile.RetryInterval.Duration())
}

func buildAuthHeader(token string) map[string]string {
	return map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
	}
}

// errMessage mirrors model.ErrMessage in the source repo — the shape of a
// downstream error response body: {"error": "..."}.
type errMessage struct {
	Error string `json:"error"`
}

// decodeHTTPError extracts a kawa.ErrInvalidHTTPStatus's status code and
// parsed error message from err. ok is false when err isn't an HTTP
// status error (e.g. a network failure) — callers must propagate err as-is
// in that case. parseErr is non-nil when the body isn't valid
// errMessage JSON.
func decodeHTTPError(err error) (statusCode int, message string, parseErr error, ok bool) {
	var errHTTP kawa.ErrInvalidHTTPStatus
	if !errors.As(err, &errHTTP) {
		return 0, "", nil, false
	}

	var em errMessage
	if unmarshalErr := json.Unmarshal(errHTTP.Body, &em); unmarshalErr != nil {
		return errHTTP.StatusCode, "", unmarshalErr, true
	}

	return errHTTP.StatusCode, em.Error, nil, true
}
