// Package httpshared holds the request/error-handling helpers shared by
// every kawa-based outbound HTTP adapter in internal/infrastructure/http
// (authremote, userremote, addressremote, productremote, basketremote,
// orderremote, payphoneremote). Each adapter package used to carry its own
// verbatim copy of deadline/retryPolicy/buildAuthHeader (and, for the ones
// that need it, decodeHTTPError) — this package is where that common
// plumbing lives instead.
//
// translateHTTPError is deliberately NOT extracted here yet: authremote's,
// addressremote's and basketremote's local copies have genuinely diverged
// (addressremote's/basketremote's version drops the status code on a
// JSON-parse failure of the response body) — reconciling that is its own
// follow-up change, not a mechanical dedup.
package httpshared

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/v8tix/beecore-eda/config"
	"github.com/v8tix/kawa"
)

// BuildAuthHeader builds the single "Authorization: Bearer <token>" header
// every adapter call in this repo sends (payphoneremote's ConfirmPayment is
// the one exception — it authenticates with PayPhone's own static token and
// an extra Content-Type header, so it keeps its own header builder).
func BuildAuthHeader(token string) map[string]string {
	return map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
	}
}

// ErrMessage mirrors model.ErrMessage in the source repo — the shape of a
// downstream error response body: {"error": "..."}.
type ErrMessage struct {
	Error string `json:"error"`
}

// DecodeHTTPError extracts a kawa.ErrInvalidHTTPStatus's status code and
// parsed error message from err. ok is false when err isn't an HTTP
// status error (e.g. a network failure) — callers must propagate err as-is
// in that case. parseErr is non-nil when the body isn't valid ErrMessage
// JSON; statusCode is still populated in that case, so callers that need
// the status code on a parse failure don't have to re-decode err
// themselves.
func DecodeHTTPError(err error) (statusCode int, message string, parseErr error, ok bool) {
	var errHTTP kawa.ErrInvalidHTTPStatus
	if !errors.As(err, &errHTTP) {
		return 0, "", nil, false
	}

	var em ErrMessage
	if unmarshalErr := json.Unmarshal(errHTTP.Body, &em); unmarshalErr != nil {
		return errHTTP.StatusCode, "", unmarshalErr, true
	}

	return errHTTP.StatusCode, em.Error, nil, true
}

// Deadline returns cfg's "fast" HTTP client profile timeout. Every adapter
// call in this repo except orderremote's GetOrdersByUserID (which keeps its
// own override) is a single-record lookup or mutation, so they all use
// this same config-tunable profile.
func Deadline(cfg *config.Cfg) time.Duration {
	return cfg.HTTPClient.GetFast().Timeout.Duration()
}

// RetryPolicy returns cfg's "fast" HTTP client profile retry policy, same
// profile as Deadline.
func RetryPolicy(cfg *config.Cfg) kawa.RetryPolicy {
	profile := cfg.HTTPClient.GetFast()
	return kawa.NewExponentialRetryPolicy(profile.MaxRetries, profile.RetryInterval.Duration())
}
