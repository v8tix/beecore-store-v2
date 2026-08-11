package kawa

import (
	"context"
	"maps"
	"net/http"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/reactivex/rxgo/v2"
)

// HTTPCall is a reusable descriptor for a typed HTTP operation.
// T is the request type (must implement Req); U is the response type (must implement Res).
//
// Construct with NewCall, then chain With* methods to configure:
//
// call := kawa.NewCall[CreateUserReq, UserRes](client, kawa.Post, "https://api.example.com/users").
//
//	WithDeadline(10 * time.Second).
//	WithRetryPolicy(kawa.NewExponentialRetryPolicy(3, 500*time.Millisecond)).
//	WithHeaders(map[string]string{"Authorization": "Bearer token"})
//
// env, err := call.Do(ctx, &req)
// obs       := call.Observable(ctx, &req)
type HTTPCall[T Req, U Res] struct {
	client   *http.Client
	method   HTTPMethod
	endpoint string
	cfg      Config
}

// NewCall creates an HTTPCall for the given client, method, and endpoint.
// Chain With* methods to configure deadline, headers, and retry behaviour.
// Production defaults are applied on the first call to Do or Observable.
func NewCall[T Req, U Res](
	client *http.Client,
	method HTTPMethod,
	endpoint string,
) *HTTPCall[T, U] {
	return &HTTPCall[T, U]{
		client:   client,
		method:   method,
		endpoint: endpoint,
	}
}

// WithDeadline sets the per-request deadline.
func (c *HTTPCall[T, U]) WithDeadline(d time.Duration) *HTTPCall[T, U] {
	c.cfg.Deadline = d
	return c
}

// WithHeaders sets default headers sent on every request.
func (c *HTTPCall[T, U]) WithHeaders(h map[string]string) *HTTPCall[T, U] {
	c.cfg.Headers = h
	return c
}

// WithRetryPolicy replaces the entire retry strategy.
func (c *HTTPCall[T, U]) WithRetryPolicy(p RetryPolicy) *HTTPCall[T, U] {
	c.cfg.Retry = p
	return c
}

// WithMaxRetries is a shortcut that sets the retry cap while keeping
// exponential backoff as the backoff strategy.
func (c *HTTPCall[T, U]) WithMaxRetries(n uint64) *HTTPCall[T, U] {
	c.cfg.Retry = exponentialRetryPolicy{maxRetries: n}
	return c
}

// WithErrorPolicy replaces the entire error-classification strategy.
func (c *HTTPCall[T, U]) WithErrorPolicy(p ErrorPolicy) *HTTPCall[T, U] {
	c.cfg.ErrPolicy = p
	return c
}

// WithClassifier sets a function-based error classifier.
func (c *HTTPCall[T, U]) WithClassifier(clf ErrorClassifier) *HTTPCall[T, U] {
	c.cfg.ErrPolicy = clf
	return c
}

// WithBodyLimit sets the maximum number of bytes read from a successful response
// body. Responses larger than this are truncated and decoding will fail with
// ErrBodySizeLimit. Defaults to 10 MB.
func (c *HTTPCall[T, U]) WithBodyLimit(n int64) *HTTPCall[T, U] {
	c.cfg.BodyLimit = n
	return c
}

// WithErrorBodyLimit sets the maximum number of bytes read from an error
// response body and stored in ErrInvalidHTTPStatus.Body. Defaults to 64 KB.
func (c *HTTPCall[T, U]) WithErrorBodyLimit(n int64) *HTTPCall[T, U] {
	c.cfg.ErrorBodyLimit = n
	return c
}

// Do execute the HTTP call synchronously and returns the typed envelope.
func (c *HTTPCall[T, U]) Do(ctx context.Context, request *T) (*Envelope[U], error) {
	cfg := c.cfg
	cfg.Headers = maps.Clone(c.cfg.Headers) // deep-copy: Headers is a map (reference type)
	applyDefaults(&cfg)
	return execute[T, U](ctx, c.client, c.method, c.endpoint, request, cfg)
}

// Observable wraps Do in a cold rxgo.Observable with retry and error classification.
//
// rxgo.Defer keeps the call lazy; BackOffRetry re-subscribes on each attempt,
// re-executing the HTTP request — correct retry semantics.
func (c *HTTPCall[T, U]) Observable(ctx context.Context, request *T) rxgo.Observable {
	cfg := c.cfg
	cfg.Headers = maps.Clone(c.cfg.Headers) // deep-copy: Headers is a map (reference type)
	applyDefaults(&cfg)
	obs := rxgo.Defer([]rxgo.Producer{
		func(_ context.Context, next chan<- rxgo.Item) {
			res, err := execute[T, U](ctx, c.client, c.method, c.endpoint, request, cfg)
			if err != nil {
				next <- rxgo.Error(cfg.ErrPolicy.Classify(err))
				return
			}
			next <- rxgo.Of(res)
		},
	})

	maxRetries := cfg.Retry.MaxRetries()
	if maxRetries == 0 {
		return obs
	}
	bo := backoff.WithContext(
		backoff.WithMaxRetries(cfg.Retry.NewBackOff(), maxRetries),
		ctx,
	)
	return obs.BackOffRetry(bo)
}

// DoWithRetry executes call synchronously with the configured RetryPolicy and
// ErrorPolicy applied — Do wrapped in Observable's retry/backoff, without
// exposing RxGo to the caller.
func (c *HTTPCall[T, U]) DoWithRetry(ctx context.Context, request *T) (*Envelope[U], error) {
	item := <-c.Observable(ctx, request).Observe()
	return ItemValue[Envelope[U]](item)
}
