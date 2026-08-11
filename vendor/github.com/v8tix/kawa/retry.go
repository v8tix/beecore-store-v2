package kawa

import (
	"errors"
	"net/http"
	"time"

	"github.com/cenkalti/backoff/v4"
)

// ── Interfaces ────────────────────────────────────────────────────────────────

// RetryPolicy controls retry behavior.
// Implement to plug in a circuit breaker, fixed-interval policy, or any custom strategy.
type RetryPolicy interface {
	// MaxRetries is the maximum number of retry attempts after the first failure.
	// Zero means one attempt only (no retries).
	MaxRetries() uint64
	// NewBackOff returns a fresh backoff.BackOff instance.
	// Called once per Rx subscription so concurrent calls never share state.
	NewBackOff() backoff.BackOff
}

// ErrorPolicy classifies errors before each retry decision.
// Return Permanent(err) to stop retrying immediately.
type ErrorPolicy interface {
	Classify(err error) error
}

// ErrorClassifier is a convenience function type that satisfies ErrorPolicy.
type ErrorClassifier func(err error) error

// Classify implements ErrorPolicy.
func (f ErrorClassifier) Classify(err error) error { return f(err) }

// Permanent wraps err so the retry loop stops immediately without consuming
// any remaining retry budget.
func Permanent(err error) error { return backoff.Permanent(err) }

// ── Default RetryPolicy ───────────────────────────────────────────────────────

type exponentialRetryPolicy struct {
	maxRetries      uint64
	initialInterval time.Duration
}

func (p exponentialRetryPolicy) MaxRetries() uint64 { return p.maxRetries }
func (p exponentialRetryPolicy) NewBackOff() backoff.BackOff {
	b := backoff.NewExponentialBackOff()
	if p.initialInterval > 0 {
		b.InitialInterval = p.initialInterval
	}
	return b
}

// NewExponentialRetryPolicy returns a RetryPolicy that uses exponential backoff
// with the given maximum retry count and initial interval.
// If initialInterval is zero, the backoff library default (500ms) is used.
func NewExponentialRetryPolicy(maxRetries uint64, initialInterval time.Duration) RetryPolicy {
	return exponentialRetryPolicy{maxRetries: maxRetries, initialInterval: initialInterval}
}

// defaultTransientCodes is the out-of-the-box retryable set.
var defaultTransientCodes = []int{
	http.StatusRequestTimeout,      // 408
	http.StatusTooManyRequests,     // 429
	http.StatusInternalServerError, // 500
	http.StatusBadGateway,          // 502
	http.StatusServiceUnavailable,  // 503
	http.StatusGatewayTimeout,      // 504
}

// HTTPStatusPolicy is an ErrorPolicy that classifies HTTP status codes as
// permanent or transient based on a configurable code table.
//
// Default transient (retryable) codes: 408, 429, 500, 502, 503, 504.
// All other 4xx codes are permanent — the same request will always produce the
// same client error, so retrying wastes the retry budget.
// Non-HTTP errors (network failures, context cancellation) are always transient.
//
// Use WithTransient / WithPermanent to override individual codes:
//
//	kawa.NewHTTPStatusPolicy().WithTransient(http.StatusNotFound)
//	kawa.NewHTTPStatusPolicy().WithPermanent(http.StatusServiceUnavailable)
type HTTPStatusPolicy struct {
	transientCodes map[int]struct{}
}

// NewHTTPStatusPolicy returns an HTTPStatusPolicy preloaded with the default
// transient code table.
func NewHTTPStatusPolicy() *HTTPStatusPolicy {
	p := &HTTPStatusPolicy{transientCodes: make(map[int]struct{}, len(defaultTransientCodes))}
	for _, code := range defaultTransientCodes {
		p.transientCodes[code] = struct{}{}
	}
	return p
}

// WithTransient adds status codes to the retryable set.
func (p *HTTPStatusPolicy) WithTransient(codes ...int) *HTTPStatusPolicy {
	for _, c := range codes {
		p.transientCodes[c] = struct{}{}
	}
	return p
}

// WithPermanent removes status codes from the retryable set, making them permanent.
func (p *HTTPStatusPolicy) WithPermanent(codes ...int) *HTTPStatusPolicy {
	for _, c := range codes {
		delete(p.transientCodes, c)
	}
	return p
}

// Classify implements ErrorPolicy.
// Any HTTP error whose status code is NOT in the transient set is wrapped in
// Permanent so the retry loop stops immediately.
// Non-HTTP errors are returned unchanged (treated as transient).
func (p *HTTPStatusPolicy) Classify(err error) error {
	var httpErr ErrInvalidHTTPStatus
	if !errors.As(err, &httpErr) || httpErr.StatusCode == 0 {
		return err
	}
	code := httpErr.StatusCode
	if code < 400 {
		return err
	}
	if _, transient := p.transientCodes[code]; !transient {
		return Permanent(err)
	}
	return err
}

// ── Config ────────────────────────────────────────────────────────────────────

// Config holds the resolved configuration for an HTTPCall.
type Config struct {
	Retry          RetryPolicy
	ErrPolicy      ErrorPolicy
	Deadline       time.Duration
	Headers        map[string]string
	BodyLimit      int64 // max bytes read from a success response body (0 = use default)
	ErrorBodyLimit int64 // max bytes read from an error response body (0 = use default)
}

func applyDefaults(c *Config) {
	if c.Retry == nil {
		c.Retry = exponentialRetryPolicy{maxRetries: 3}
	}
	if c.ErrPolicy == nil {
		c.ErrPolicy = NewHTTPStatusPolicy()
	}
	if c.Deadline == 0 {
		c.Deadline = DefaultTimeout
	}
	if c.BodyLimit == 0 {
		c.BodyLimit = maxResponseBodyBytes
	}
	if c.ErrorBodyLimit == 0 {
		c.ErrorBodyLimit = maxErrorBodyBytes
	}
}
