// Package httpshared holds the request/error-handling helpers shared by
// every kawa-based outbound HTTP adapter in internal/infrastructure/http
// (authremote, userremote, addressremote, productremote, basketremote,
// orderremote, payphoneremote). Each adapter package used to carry its own
// verbatim copy of deadline/retryPolicy/buildAuthHeader (and, for the ones
// that need it, decodeHTTPError/translateHTTPError) — this package is
// where that common plumbing lives instead.
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

// TranslateHTTPError turns a kawa.ErrInvalidHTTPStatus into a plain error
// carrying the downstream status code plus a description of the response
// body, for call sites that don't need to distinguish specific downstream
// rejection reasons. The status code is always preserved, even when the
// body isn't valid ErrMessage JSON — in that case the raw body is used as
// the description instead of the (would-be) parsed message. Non-HTTP
// errors (network failures) pass through unchanged.
//
// This is the single reconciled behavior for what used to be 3 diverged
// per-adapter copies: authremote's always wrapped the raw body (which this
// keeps as the parse-failure fallback); addressremote's and basketremote's
// extracted only the parsed message and, on a parse failure, returned the
// bare json.Unmarshal error — silently discarding the status code and body.
// That silent loss is judged a real, debugging-hostile bug, not a
// behavior worth preserving, so it isn't reproduced here.
func TranslateHTTPError(err error) error {
	var errHTTP kawa.ErrInvalidHTTPStatus
	if !errors.As(err, &errHTTP) {
		return err
	}

	var em ErrMessage
	if unmarshalErr := json.Unmarshal(errHTTP.Body, &em); unmarshalErr == nil {
		return fmt.Errorf("downstream request failed with status %d: %s", errHTTP.StatusCode, em.Error)
	}

	return fmt.Errorf("downstream request failed with status %d: %s", errHTTP.StatusCode, string(errHTTP.Body))
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
