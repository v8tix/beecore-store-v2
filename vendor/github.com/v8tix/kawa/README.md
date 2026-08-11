# kawa 川

> A reactive HTTP client for Go. Typed calls, observable streams, and configurable retry that flows around failure.

[![Go Reference](https://pkg.go.dev/badge/github.com/v8tix/kawa.svg)](https://pkg.go.dev/github.com/v8tix/kawa)
![Go version](https://img.shields.io/badge/go-1.26-blue)

---

## Why kawa?

Most HTTP clients make you choose between simplicity and power. kawa gives you both:

- **Typed generics** — no `interface{}` casting, compile-time safety end-to-end
- **Observable streams** via [RxGo v2](https://github.com/ReactiveX/RxGo) — compose parallel requests, fan-out, pipelines
- **Smart retry by default** — 4xx errors stop immediately (no wasted budget); 5xx and network errors retry with exponential backoff
- **Fully configurable** — swap the retry policy, tweak which status codes are retryable, set per-call deadlines and body limits

---

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [Request & Response Types](#request--response-types)
- [Building Calls](#building-calls)
- [Sync Execution — `Do`](#sync-execution--do)
- [Sync Execution With Retry — `DoWithRetry`](#sync-execution-with-retry--dowithretry)
- [Reactive Execution — `Observable`](#reactive-execution--observable)
- [Retry Policy](#retry-policy)
- [Error Policy](#error-policy)
- [Error Types](#error-types)
- [HTTP Client Setup](#http-client-setup)
- [Middleware](#middleware)
- [Testing](#testing)
- [Contributing](#contributing)
- [License](#license)

---

## Installation

```bash
go get github.com/v8tix/kawa
```

---

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/v8tix/kawa"
    "github.com/v8tix/kawa/policy"
    "github.com/v8tix/kawa/transport"
)

// Step 1 — define your domain types.
type CreateUserReq struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}
func (CreateUserReq) Req() {} // implements kawa.Req

type UserRes struct {
    ID   string `json:"id"`
    Name string `json:"name"`
}
func (UserRes) Res() {} // implements kawa.Res

func main() {
    // Step 2 — build a shared HTTP client once.
    client := kawa.NewHTTPClient(
        5*time.Second,
        policy.OneRedirect,
        transport.IdleConnectionTimeout(kawa.DefaultTimeout),
    )

    // Step 3 — describe the call once; reuse it everywhere.
    createUser := kawa.NewCall[CreateUserReq, UserRes](client, kawa.Post, "https://api.example.com/users").
        WithDeadline(10 * time.Second).
        WithMaxRetries(3)

    // Step 4 — execute synchronously.
    env, err := createUser.Do(context.Background(), &CreateUserReq{
        Name:  "Ada Lovelace",
        Email: "ada@example.com",
    })
    if err != nil {
        panic(err)
    }
    fmt.Println("created user:", env.Body.ID)
}
```

---

## Request & Response Types

kawa uses two marker interfaces to keep the type system honest:

```go
// Req marks a type as a valid HTTP request body.
type Req interface{ Req() }

// Res marks a type as a valid HTTP response body.
type Res interface{ Res() }
```

Every type you pass to `NewCall` must satisfy the appropriate interface:

```go
type CreateOrderReq struct {
    ProductID string `json:"product_id"`
    Qty       int    `json:"qty"`
}
func (CreateOrderReq) Req() {}

type OrderRes struct {
    ID     string `json:"id"`
    Status string `json:"status"`
}
func (OrderRes) Res() {}
```

### Sentinel types for empty bodies

When a call has no request body (GET) or no response body (DELETE returning 204), use the built-in sentinels instead of `nil`:

```go
// No request body
kawa.NewCall[kawa.NoReq, OrderRes](client, kawa.Get, url)

// No response body (e.g. 204 No Content)
kawa.NewCall[CreateOrderReq, kawa.NoRes](client, kawa.Delete, url)
```

### Query string requests

If your request type also implements `ReqURLI`, kawa encodes it as a URL query string instead of a JSON body — useful for GET requests with filter parameters:

```go
type SearchReq struct {
    Query string
    Page  int
}
func (SearchReq) Req() {}
func (r SearchReq) URLValues() url.Values {
    v := url.Values{}
    v.Set("q", r.Query)
    v.Set("page", strconv.Itoa(r.Page))
    return v
}
```

---

## Building Calls

`NewCall[T, U]` creates a reusable `*HTTPCall[T, U]` descriptor. Configure it once with `With*` methods, then call `Do` or `Observable` as many times as needed — each invocation gets its own deep-copied config so concurrent calls never share state.

```go
getUser := kawa.NewCall[kawa.NoReq, UserRes](client, kawa.Get, "https://api.example.com/users/"+id).
    WithDeadline(5 * time.Second).
    WithHeaders(map[string]string{"Authorization": "Bearer " + token}).
    WithMaxRetries(2)
```

### Builder options

| Method | Default | Description |
|--------|---------|-------------|
| `WithDeadline(d time.Duration)` | `15s` | Per-request timeout. Fires after `d` regardless of the parent context. |
| `WithHeaders(h map[string]string)` | `nil` | Extra headers merged into every request. |
| `WithMaxRetries(n uint64)` | `3` | Shorthand to set max retries while keeping exponential backoff. |
| `WithRetryPolicy(p RetryPolicy)` | Exponential backoff, 3 retries | Full retry strategy replacement — implement the interface to plug in a circuit breaker, fixed-delay policy, etc. |
| `WithErrorPolicy(p ErrorPolicy)` | `HTTPStatusPolicy` | Controls which errors are retried vs. stopped immediately. |
| `WithClassifier(fn ErrorClassifier)` | `HTTPStatusPolicy` | Function shorthand for `WithErrorPolicy` — `func(err error) error`. |
| `WithBodyLimit(n int64)` | `10 MB` | Max bytes read from a **success** response body. Truncation causes a decode error. |
| `WithErrorBodyLimit(n int64)` | `64 KB` | Max bytes stored in `ErrInvalidHTTPStatus.Body` from an error response. |

---

## Sync Execution — `Do`

`Do` executes the HTTP call synchronously and returns a typed `*Envelope[U]`:

```go
env, err := createUser.Do(ctx, &req)
if err != nil {
    // See "Error Types" below for what err can be
}

// env.Body is *UserRes — decoded JSON response
fmt.Println(env.Body.ID)

// env.Response is *http.Response — headers, status code, raw body
fmt.Println(env.StatusCode)

// The raw body is still readable after Do returns
raw, _ := io.ReadAll(env.Response.Body)
```

> `env.Response.Body` is replaced with a re-readable `io.NopCloser` over the buffered bytes, so you can inspect it after the network connection is already closed.

---

## Sync Execution With Retry — `DoWithRetry`

`Do` never retries — it's a single-shot call. Retry only exists on the `Observable` path, which requires unpacking an `rxgo.Item` even if you don't want reactive composition. `DoWithRetry` is that unpacking done for you: `Do`'s call shape, `Observable`'s retry/backoff.

```go
env, err := createUser.WithRetryPolicy(kawa.NewExponentialRetryPolicy(3, 200*time.Millisecond)).
    DoWithRetry(ctx, &req)
if err != nil {
    // See "Error Types" below for what err can be
}

fmt.Println(env.Body.ID)
```

Equivalent to:

```go
item := <-createUser.Observable(ctx, &req).Observe()
env, err := kawa.ItemValue[Envelope[UserRes]](item)
```

Use `Do` when you don't want retry at all. Use `DoWithRetry` for the common case: synchronous call, retry on transient failure, no RxGo composition needed. Use `Observable` directly when you need fan-out, merge, or pipelining.

---

## Reactive Execution — `Observable`

`Observable` wraps `Do` in a cold [RxGo](https://github.com/ReactiveX/RxGo) observable with automatic retry. It emits one `*Envelope[U]` on success, or terminates with an error once retries are exhausted.

```go
items := getUser.Observable(ctx, nil).Observe()

item, ok := <-items
if !ok {
    // channel closed without an item
}

env, err := kawa.ItemValue[Envelope[UserRes]](item)
if err != nil {
    // handle
}
fmt.Println(env.Body.ID)
```

### Composing observables

Because it's a standard RxGo observable, you can fan-out, merge, and pipeline:

```go
// Fire two requests in parallel
obs1 := fetchUser.Observable(ctx, nil)
obs2 := fetchOrders.Observable(ctx, nil)

for item := range rxgo.Merge([]rxgo.Observable{obs1, obs2}).Observe() {
    if item.E != nil {
        log.Println("error:", item.E)
        continue
    }
    // item.V is *Envelope[T] — type-assert to the appropriate type
}
```

### `ItemValue` helper

`ItemValue[T]` safely unpacks an `rxgo.Item` and handles all error cases:

```go
env, err := kawa.ItemValue[Envelope[UserRes]](item)
```

| Item state | Result |
|---|---|
| `item.V` is `*T` | Returns `*T, nil` |
| `item.E` is set | Returns `nil, item.E` |
| `item.V` is `nil` and `item.E` is `nil` | Returns `nil, ErrEmptyItem` |
| `item.V` is a non-matching type | Returns `nil, ErrNonPointerOrWrongCasting` |

---

## Retry Policy

The retry policy controls **how many times** and **how long** to wait between attempts.

### Default: exponential backoff

Out of the box, kawa retries up to **3 times** with exponential backoff (initial interval: 500 ms, multiplied by 1.5× each attempt).

```go
// Override just the count
call.WithMaxRetries(5)

// Override count and initial interval
call.WithRetryPolicy(
    kawa.NewExponentialRetryPolicy(5, 200*time.Millisecond),
)
```

### Custom policy

Implement `RetryPolicy` to plug in any strategy — fixed interval, circuit breaker, jitter, etc.:

```go
type FixedRetryPolicy struct{}

func (FixedRetryPolicy) MaxRetries() uint64 { return 3 }
func (FixedRetryPolicy) NewBackOff() backoff.BackOff {
    return backoff.NewConstantBackOff(100 * time.Millisecond)
}

call.WithRetryPolicy(FixedRetryPolicy{})
```

`NewBackOff()` is called once per Observable subscription, so concurrent calls never share backoff state.

### Retry policy options

| Constructor | Description |
|---|---|
| `NewExponentialRetryPolicy(n, interval)` | Exponential backoff; `interval=0` uses the library default (500 ms). |
| Implement `RetryPolicy` | Any custom strategy — fixed delay, circuit breaker, jitter. |

---

## Error Policy

The error policy controls **which errors are retried** and which stop the loop immediately.

### Default: `HTTPStatusPolicy`

kawa's default policy classifies HTTP errors by status code:

| Status code(s) | Classification | Reason |
|---|---|---|
| 408 Request Timeout | **Transient** — retried | Server-side timeout, worth retrying |
| 429 Too Many Requests | **Transient** — retried | Rate-limited; backoff helps |
| 500 Internal Server Error | **Transient** — retried | Likely transient fault |
| 502 Bad Gateway | **Transient** — retried | Upstream unavailable |
| 503 Service Unavailable | **Transient** — retried | Deployment / overload |
| 504 Gateway Timeout | **Transient** — retried | Upstream timeout |
| All other 4xx (400, 401, 403, 404…) | **Permanent** — stops immediately | Client error; retrying wastes the budget |
| Network / context errors | **Transient** — retried | Connectivity blip |

### Customising `HTTPStatusPolicy`

```go
// Make 404 retryable — useful when a resource is provisioned asynchronously
policy := kawa.NewHTTPStatusPolicy().WithTransient(http.StatusNotFound)

// Treat 503 as permanent — stop immediately on maintenance windows
policy := kawa.NewHTTPStatusPolicy().WithPermanent(http.StatusServiceUnavailable)

// Chain multiple overrides
policy := kawa.NewHTTPStatusPolicy().
    WithTransient(http.StatusNotFound, http.StatusConflict).
    WithPermanent(http.StatusServiceUnavailable)

call.WithErrorPolicy(policy)
```

### Function-based classifier

For simple cases, skip the struct and use an inline function:

```go
call.WithClassifier(kawa.ErrorClassifier(func(err error) error {
    if errors.Is(err, ErrRateLimited) {
        return err // transient — retry
    }
    return kawa.Permanent(err) // stop immediately
}))
```

### `kawa.Permanent`

Wrap any error with `kawa.Permanent(err)` to stop the retry loop immediately, regardless of the active policy. The underlying error is preserved and unwrappable with `errors.Is` / `errors.As`.

---

## Error Types

| Sentinel / type | When returned |
|---|---|
| `ErrTimeOut` | The **per-request deadline** (`WithDeadline`) fired — either before the server responded, or while its response body was still being read. Distinct from a caller-cancelled context. |
| `ErrInvalidHTTPStatus` | Server replied with a 4xx or 5xx. Carries `.StatusCode`, `.Status`, and `.Body` (up to `WithErrorBodyLimit`). |
| `ErrMarshalValue` | Request body could not be marshalled to JSON. |
| `ErrNilValue` | A non-nil pointer was required but `nil` was passed. |
| `ErrNonPointerOrWrongCasting` | `ItemValue[T]` found a value of the wrong type in the item. |
| `ErrEmptyItem` | `ItemValue[T]` received an item with no value and no error. |

### Inspecting `ErrInvalidHTTPStatus`

```go
env, err := call.Do(ctx, &req)
if err != nil {
    var httpErr kawa.ErrInvalidHTTPStatus
    if errors.As(err, &httpErr) {
        fmt.Println(httpErr.StatusCode) // e.g. 422
        fmt.Println(string(httpErr.Body)) // error payload from the server
    }
}
```

---

## HTTP Client Setup

kawa works with any `*http.Client`. The helpers below make production setup ergonomic:

```go
import (
    "github.com/v8tix/kawa"
    "github.com/v8tix/kawa/policy"
    "github.com/v8tix/kawa/transport"
)

client := kawa.NewHTTPClient(
    5*time.Second,                                   // client-level timeout (hard ceiling)
    policy.OneRedirect,                              // allow at most one redirect
    transport.IdleConnectionTimeout(kawa.DefaultTimeout), // keep-alive connection reuse
)
```

### `NewHTTPClient` parameters

| Parameter | Description |
|---|---|
| `timeout time.Duration` | Hard client-level timeout — fires even if the call's own deadline hasn't expired. |
| `redirectPolicy func(*http.Request, []*http.Request) error` | Called by `net/http` before each redirect. Use `policy.OneRedirect` or `nil` for default (10 redirects). |
| `transport http.RoundTripper` | The underlying transport. Wrap with middleware (see below). |

### `transport.IdleConnectionTimeout`

```go
// Returns a *http.Transport configured with keep-alive idle timeout
transport.IdleConnectionTimeout(30 * time.Second)
```

### `policy.OneRedirect`

Stops after the first redirect and returns `policy.ErrMoreThanOneRedirect` — prevents redirect loops in microservice meshes.

---

## Middleware

Middleware wraps `http.RoundTripper` to add cross-cutting concerns to every request transparently.

### `middleware.LoggingTransport`

Logs every request and response using structured [`log/slog`](https://pkg.go.dev/log/slog):

```go
import (
    "log/slog"
    "github.com/v8tix/kawa/middleware"
    "github.com/v8tix/kawa/transport"
)

inner := transport.IdleConnectionTimeout(kawa.DefaultTimeout)
logged := middleware.NewLoggingTransport(logger, inner).
    WithLevel(slog.LevelDebug).       // request + response log level (default: Info)
    WithErrorLevel(slog.LevelWarn)    // error log level (default: Error)

client := kawa.NewHTTPClient(5*time.Second, policy.OneRedirect, logged)
```

#### `LoggingTransport` options

| Method | Default | Description |
|---|---|---|
| `WithLevel(lvl slog.Level)` | `slog.LevelInfo` | Log level for outgoing requests and incoming responses. |
| `WithErrorLevel(lvl slog.Level)` | `slog.LevelError` | Log level when the round-trip fails. |

If `logger` is `nil`, `slog.Default()` is used. If `next` is `nil`, `http.DefaultTransport` is used.

### `middleware.CustomHeaders`

Injects a fixed set of headers into every outgoing request — ideal for API keys and tenant identifiers:

```go
import "github.com/v8tix/kawa/middleware"

withAuth := middleware.NewCustomHeaders(
    map[string]string{
        "X-Api-Key":  os.Getenv("API_KEY"),
        "X-Tenant":   tenantID,
    },
    transport.IdleConnectionTimeout(kawa.DefaultTimeout), // wraps next transport
)

client := kawa.NewHTTPClient(5*time.Second, policy.OneRedirect, withAuth)
```

### Composing middleware

Middleware wraps each other in layers — outermost wraps innermost:

```go
inner   := transport.IdleConnectionTimeout(kawa.DefaultTimeout)
headers := middleware.NewCustomHeaders(map[string]string{"X-Api-Key": key}, inner)
logged  := middleware.NewLoggingTransport(logger, headers)

client := kawa.NewHTTPClient(5*time.Second, policy.OneRedirect, logged)
// request flow: logged → headers → inner → network
```

---

## Testing

Run the full suite with the race detector:

```bash
make test
# equivalent: go clean -testcache && go test -count=1 -race -timeout 60s ./...
```

Tests are split into two layers:

- **White-box** (`package kawa` in the root): unit tests for unexported helpers — `sendRequest`, `marshalToReader`, `buildReader`, `newErrInvalidHTTPStatus`.
- **Behavioral** (`package tests` under `tests/`): black-box tests against the public API, grouped by concern. Each directory shares a `TestMain` for server lifecycle and helper types.

### `tests/http` — HTTP call behaviour

| Test | Description |
|---|---|
| `TestNewCallDoReturnsResponseOnSuccess` | `Do` returns a populated `Envelope` on a 200 response. |
| `TestNewCallDoReturns404AsPermanentError` | `Do` returns `ErrInvalidHTTPStatus` with status 404; default policy marks it permanent. |
| `TestNewCallDoWithCustomHeadersSendsHeaders` | Headers set via `WithHeaders` are forwarded on every request. |
| `TestNewCallDoAllMethodsWithBodySucceed` | GET / POST / PUT / PATCH / DELETE all complete without error when given a body. |
| `TestNewCallDoDeadlineExceededReturnsErrTimeOut` | `WithDeadline` fires before the server responds; result is `ErrTimeOut` (not a context error). |
| `TestEnvelopeResponseBodyIsReadableAfterDo` | `env.Response.Body` remains readable after `Do` closes the network connection. |
| `TestNewCallObservableReturnsResponseOnSuccess` | `Observable` emits one item containing the typed response. |
| `TestNewCallObservableRetriesTransientErrorsThenSucceeds` | Transient errors trigger retries; the call succeeds once the server recovers. |
| `TestNewCallObservableDoesNotRetryPermanent404` | Permanent 404 is emitted immediately without consuming the retry budget. |
| `TestNewCallObservableWith404MadeTransientDoesRetry` | `WithErrorPolicy` can promote 404 to transient; retries are then observed. |
| `TestNewCallObservableAllMethodsWithNilBodySucceed` | All HTTP methods work with a `nil` request body on the Observable path. |
| `TestNewCallObservableAllMethodsWithBodySucceed` | All HTTP methods work with a request body on the Observable path. |
| `TestHTTPCallDoConcurrentCallsDoNotRaceOnHeaders` | Concurrent `Do` calls on the same `*HTTPCall` share no mutable state (race-detector clean). |
| `TestWithClassifierIsUsedByObservable` | A custom `ErrorClassifier` function is invoked; its return value replaces the error emitted. |
| `TestWithBodyLimitTruncatesLargeSuccessBody` | `WithBodyLimit` truncates oversized success bodies, causing a decode error. |
| `TestWithErrorBodyLimitCapsErrorBody` | `WithErrorBodyLimit` caps bytes stored in `ErrInvalidHTTPStatus.Body`. |
| `TestErrorClassifierImplementsErrorPolicy` | `ErrorClassifier` (function type) satisfies the `ErrorPolicy` interface. |
| `TestItemValueWithValidPointerReturnsValue` | `ItemValue` unpacks a correctly typed item. |
| `TestItemValueWithNonPointerReturnsErrNonPointer` | Non-pointer values in an item return `ErrNonPointerOrWrongCasting`. |
| `TestItemValueWithErrorValueReturnsError` | `item.V` holding an error is forwarded as the return error. |
| `TestItemValueWithNilValueReturnsErrEmptyItem` | Empty item (no value, no error) returns `ErrEmptyItem`. |
| `TestItemValueWithNilValueAndErrorReturnsError` | `item.E` is returned directly when `item.V` is nil. |
| `TestItemValueWithWrongTypeReturnsErrNonPointer` | Type mismatch between the generic parameter and the item value returns `ErrNonPointerOrWrongCasting`. |
| `TestExecuteBodyIOFailures` | I/O errors on response body read/close are surfaced correctly; original body is always closed. |
| `TestNewCallDoLargeResponseBodySucceeds` | `Do` succeeds against a large, multi-KB response body (a growing list, not a single object). |
| `TestNewCallDoWithRetryLargeResponseBodySucceeds` | `DoWithRetry` succeeds against the same large-body shape. |
| `TestNewCallDoDeadlineExceededDuringBodyReadReturnsErrTimeOut` | `WithDeadline` firing mid-body-stream (not just before headers) still translates to `ErrTimeOut`, not a raw context error. |
| `TestNewCallDoCallerCancelDuringBodyReadReturnsContextDeadlineExceeded` | A caller-driven cancellation mid-body-read surfaces as the caller's own context error, not `ErrTimeOut`. |
| `TestNewCallDoWithRetryLargeResponseBodyAfterTransientTimeoutSucceeds` | A timed-out first attempt followed by a fast large-body second attempt succeeds — retries don't leak release/timer state across attempts. |
| `TestEnvelopeResponseBodyIsReadableAfterDoWithLargeBody` | `env.Response.Body` remains independently re-readable after `Do` for large payloads too. |
| `TestHTTPCallDoConcurrentCallsWithLargeBodiesDoNotRace` | Concurrent `Do` calls with large response bodies share no mutable state (race-detector clean). |

### `tests/retry` — Retry and error policy

| Test | Description |
|---|---|
| `TestHTTPStatusPolicy_DefaultCodeClassification` | 408/429/5xx are transient; other 4xx are permanent out of the box. |
| `TestHTTPStatusPolicy_OverrideClassification` | `WithTransient` / `WithPermanent` override individual status codes. |
| `TestHTTPStatusPolicy_NonHTTPErrorIsAlwaysTransient` | Non-HTTP errors (network failures) are never wrapped as permanent. |
| `TestHTTPStatusPolicy_ZeroStatusCodeIsTransient` | A zero status code (no HTTP error) is not classified as permanent. |
| `TestHTTPStatusPolicy_WithTransientChaining` | Multiple `WithTransient` calls accumulate correctly. |
| `TestHTTPStatusPolicy_PermanentPreservesUnderlyingError` | `kawa.Permanent(err)` wraps err; `errors.Is` still unwraps to the original. |
| `TestNewExponentialRetryPolicy_MaxRetries` | Zero disables retries; configured values are returned faithfully. |
| `TestNewExponentialRetryPolicy_NewBackOffReturnsFreshInstance` | Each `NewBackOff()` call returns a distinct instance — safe for concurrent use. |
| `TestNewExponentialRetryPolicy_InitialInterval` | Custom interval is applied; zero falls back to the library default (500 ms). |
| `TestMarkerMethodsDoNotPanic` | `NoReq.Req()` and `NoRes.Res()` are callable and do not panic. |
| `TestErrInvalidHTTPStatusErrorMethod` | `ErrInvalidHTTPStatus.Error()` produces valid JSON with status code, status, and body. |

### `tests/middleware` — Transport middleware

| Test | Description |
|---|---|
| `TestNewLoggingTransport_NilArgsFallbackToDefaults` | `nil` logger → `slog.Default()`; `nil` next → `http.DefaultTransport`. |
| `TestLoggingTransport_RoundTripSuccess` | Successful round-trip logs request dispatched and response received. |
| `TestLoggingTransport_RoundTripError` | Failed round-trip logs the error at the configured error level. |
| `TestLoggingTransport_WithLevelOptionsDoNotPanic` | `WithLevel` and `WithErrorLevel` accept all slog levels without panicking. |
| `TestCustomHeaders_InjectsHeadersIntoRequest` | Configured headers appear on the forwarded request. |
| `TestCustomHeaders_NilNextUsesDefaultTransport` | `nil` next falls back to `http.DefaultTransport`. |
| `TestCustomHeaders_DoesNotMutateOriginalRequest` | Original `*http.Request` is cloned; the caller's request is never modified. |

### `tests/policy` — Redirect policy

| Test | Description |
|---|---|
| `TestOneRedirect_AllowsFirstRedirect` | A single redirect (one prior request) returns `nil`. |
| `TestOneRedirect_BlocksSecondRedirect` | Two prior requests return `ErrMoreThanOneRedirect`. |
| `TestOneRedirect_AllowsNoRedirect` | An empty via-chain (no prior requests) returns `nil`. |

### `tests/transport` — Transport configuration

| Test | Description |
|---|---|
| `TestIdleConnectionTimeout_ReturnsTransportWithConfiguredTimeout` | `IdleConnectionTimeout(d)` sets `Transport.IdleConnTimeout` to `d`. |
| `TestIdleConnectionTimeout_ZeroDurationIsAllowed` | Zero duration is accepted and stored without error. |
| `TestIdleConnectionTimeout_EachCallReturnsNewInstance` | Each call returns an independent `*http.Transport` — not a shared singleton. |

---

## Contributing

1. Fork the repository
2. Clone your fork: `git clone https://github.com/your-user/kawa`
3. Create a branch: `git checkout -b feat/my-feature`
4. Make your changes and add tests
5. Run the suite: `make test`
6. Open a pull request 🎉

---

## License

Licensed under the [MIT License](LICENSE).
