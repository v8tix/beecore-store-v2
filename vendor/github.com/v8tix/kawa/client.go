package kawa

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/reactivex/rxgo/v2"
	"github.com/v8tix/jsonx"
)

const (
	Get            = HTTPMethod("GET")
	Post           = HTTPMethod("POST")
	Put            = HTTPMethod("PUT")
	Patch          = HTTPMethod("PATCH")
	Delete         = HTTPMethod("DELETE")
	DefaultTimeout = 15 * time.Second

	// maxErrorBodyBytes caps the response body stored in ErrInvalidHTTPStatus.
	// Prevents a rogue server from exhausting memory on error responses.
	maxErrorBodyBytes = 64 * 1024 // 64 KB

	// maxResponseBodyBytes caps the success response body read in execute.
	maxResponseBodyBytes = 10 * 1024 * 1024 // 10 MB
)

var (
	ErrTimeOut                  = kawaErr("timeout occurred")
	ErrMarshalValue             = kawaErr("failed to marshal the value to JSON")
	ErrBadRequest               = kawaErr("bad request")
	ErrNonPointerOrWrongCasting = kawaErr("value is not a pointer or casting type is incorrect")
	ErrEmptyItem                = kawaErr("item has no value and no error")
	ErrNilValue                 = kawaErr("nil value provided")
)

type (
	Req interface{ Req() }
	Res interface{ Res() }

	ReqURLI interface {
		URLValues() interface{ Encode() string }
	}

	// NoReq is a sentinel request type for HTTP calls with no request body (e.g. GET).
	NoReq string
	// NoRes is a sentinel response type for HTTP calls with no response body (e.g. 204 No Content).
	NoRes string

	HTTPMethod string

	// Envelope wraps the decoded response body together with the underlying
	// *http.Response for status code and header access.
	//
	// Note: Response.Body has already been read and closed by the time the caller
	// receives Envelope. It is replaced with an io.NopCloser over the raw bytes so
	// that callers can still read the original response payload if needed.
	Envelope[T Res] struct {
		Body *T
		*http.Response
	}

	kawaErr string

	// ErrInvalidHTTPStatus is returned when the server replies with a 4xx or 5xx
	// status. The response body is eagerly read (up to maxErrorBodyBytes) and stored
	// so that Error() is idempotent and safe to call multiple times.
	ErrInvalidHTTPStatus struct {
		StatusCode int
		Status     string
		Body       []byte
	}
)

// Req satisfies Req. NoReq carries no data; the method exists solely to
// let the type system express "this call has no request body" without nil.
func (nrq NoReq) Req() { /* marker — no behavior required */ }

// Res satisfies Res. NoRes carries no data; the method exists solely to
// let the type system express "this call returns no response body" (e.g. 204 No Content).
func (nrp NoRes) Res()          { /* marker — no behavior required */ }
func (e kawaErr) Error() string { return string(e) }

// newErrInvalidHTTPStatus reads and closes res.Body (up to errorBodyLimit bytes) so
// the error value is self-contained and safe to inspect repeatedly.
// If the body cannot be fully read, the partial content is retained and a note
// about the read failure is appended so the caller is not silently misled.
func newErrInvalidHTTPStatus(res *http.Response, errorBodyLimit int64) ErrInvalidHTTPStatus {
	body, readErr := io.ReadAll(io.LimitReader(res.Body, errorBodyLimit))
	closeErr := res.Body.Close()

	switch {
	case readErr != nil && len(body) == 0:
		body = []byte(fmt.Sprintf("<error reading body: %s>", readErr))
	case readErr != nil:
		body = append(body, []byte(fmt.Sprintf(" <body truncated: %s>", readErr))...)
	case closeErr != nil:
		// Body was fully read; close failure doesn't affect the content.
	}

	return ErrInvalidHTTPStatus{
		StatusCode: res.StatusCode,
		Status:     res.Status,
		Body:       body,
	}
}

func (e ErrInvalidHTTPStatus) Error() string {
	b, _ := json.Marshal(struct {
		StatusCode int    `json:"status_code,omitempty"`
		Status     string `json:"status,omitempty"`
		ErrMessage string `json:"error_message,omitempty"`
	}{
		StatusCode: e.StatusCode,
		Status:     e.Status,
		ErrMessage: string(e.Body),
	})
	return string(b)
}

// NewHTTPClient creates a pre-configured http.Client.
func NewHTTPClient(
	timeout time.Duration,
	redirectPolicy func(req *http.Request, via []*http.Request) error,
	transport http.RoundTripper,
) *http.Client {
	return &http.Client{
		Timeout:       timeout,
		Transport:     transport,
		CheckRedirect: redirectPolicy,
	}
}

// ItemValue extracts a typed value from an rxgo.Item.
func ItemValue[T any](item rxgo.Item) (*T, error) {
	switch item.V.(type) {
	case *T:
		return item.V.(*T), nil
	case nil:
		if item.E != nil {
			return nil, item.E
		}
		return nil, ErrEmptyItem
	case error:
		return nil, item.V.(error)
	default:
		return nil, ErrNonPointerOrWrongCasting
	}
}

// ── internal helpers ──────────────────────────────────────────────────────────

func defaultInvalidStatusCodeValidator(response *http.Response) bool {
	return response.StatusCode >= 400
}

// sanitizeURL returns scheme+host+path only, stripping query and fragment to
// avoid leaking auth tokens or other sensitive query parameters in error
// messages. Falls back to the raw string if it doesn't parse as a URL.
func sanitizeURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

func execute[T Req, U Res](
	ctx context.Context,
	client *http.Client,
	method HTTPMethod,
	endpoint string,
	request *T,
	cfg Config,
) (env *Envelope[U], err error) {
	reader, err := buildReader[T](request)
	if err != nil {
		return nil, err
	}

	// Avoid passing a typed nil (*bytes.Reader) as io.Reader — a non-nil interface
	// with a nil concrete value would panic inside http.NewRequestWithContext.
	var bodyReader io.Reader
	if reader != nil {
		bodyReader = reader
	}

	httpResponse, release, err := sendRequest(ctx, client, endpoint, cfg.Headers, bodyReader, cfg.Deadline, method, defaultInvalidStatusCodeValidator, cfg.ErrorBodyLimit)
	if err != nil {
		return nil, err
	}
	// release cancels the request context and stops the deadline timer. It must
	// stay live until the body is fully read below — sendRequest used to cancel
	// it the moment the round-trip finished, before the body (read here) had
	// been consumed. Small bodies are already fully buffered by then and never
	// noticed; a large one isn't, and the read raced an already-canceled
	// context and failed with "context canceled" every time.
	defer release()
	// Capture original body before we potentially replace it below, so the defer
	// always closes the underlying network connection — not a NopCloser over bytes.
	originalBody := httpResponse.Body
	defer func() {
		if closeErr := originalBody.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	if httpResponse.StatusCode == http.StatusNoContent {
		return newResponse[U](nil, httpResponse), nil
	}

	// Read the body into memory so we can:
	//   1. Decode it into the typed response.
	//   2. Restore it on Envelope.Response.Body for callers who need the raw bytes.
	rawBody, err := io.ReadAll(io.LimitReader(httpResponse.Body, cfg.BodyLimit))
	if err != nil {
		err = wrapIfOwnTimeout(err, ctx, endpoint)
		return nil, err
	}

	body, err := jsonx.ReadJSONAs[U](bytes.NewReader(rawBody))
	if err != nil {
		return nil, err
	}

	// Replace body with a re-readable NopCloser so Envelope.Response.Body works.
	httpResponse.Body = io.NopCloser(bytes.NewReader(rawBody))
	return newResponse[U](&body, httpResponse), nil
}

func newResponse[T Res](body *T, res *http.Response) *Envelope[T] {
	return &Envelope[T]{Body: body, Response: res}
}

// wrapIfOwnTimeout distinguishes our own timer-induced cancellation from a
// deliberate caller cancellation, translating the former into the friendly
// ErrTimeOut. Our timer fires cancel() on a context derived from ctx but
// leaves ctx itself untouched, so ctx.Err() == nil exactly when our timer
// fired rather than the caller's own context expiring or being canceled.
//
// Applies to errors from both phases that share the same request context:
// the round-trip itself (wrapped in *url.Error by net/http) and the body
// read that happens afterward (returned bare, not *url.Error-wrapped). rawURL
// is the original request target, not *http.Request.URL — a response from a
// hand-built or test RoundTripper may have no associated Request at all.
func wrapIfOwnTimeout(err error, ctx context.Context, rawURL string) error {
	isCancellation := errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
	if urlErr, ok := errors.AsType[*url.Error](err); ok {
		isCancellation = errors.Is(urlErr.Err, context.Canceled) || errors.Is(urlErr.Err, context.DeadlineExceeded)
	}
	if isCancellation && ctx.Err() == nil {
		return fmt.Errorf("service at %q %w", sanitizeURL(rawURL), ErrTimeOut)
	}
	return err
}

// sendRequest performs the round-trip and returns a release func the caller
// must invoke once fully done with the response — including reading its
// body. release cancels the request context and stops the deadline timer;
// on every error path here the response (if any) is already fully consumed,
// so release is safe to call immediately in those cases.
func sendRequest(
	ctx context.Context,
	client *http.Client,
	rawURL string,
	headers map[string]string,
	body io.Reader,
	deadline time.Duration,
	method HTTPMethod,
	statusCodeValidator func(res *http.Response) bool,
	errorBodyLimit int64,
) (res *http.Response, release func(), err error) {
	withCancelCtx, cancel := context.WithCancel(ctx)
	timer := time.AfterFunc(deadline, cancel)
	release = func() {
		cancel() // always release the child context; calling cancel twice is safe
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	}

	request, err := http.NewRequestWithContext(withCancelCtx, string(method), rawURL, body)
	if err != nil {
		release()
		return nil, release, err
	}
	for k, v := range headers {
		request.Header.Add(k, v)
	}

	httpRes, err := client.Do(request)
	if err != nil {
		release()
		return nil, release, wrapIfOwnTimeout(err, ctx, rawURL)
	}
	if statusCodeValidator(httpRes) {
		release()
		return nil, release, newErrInvalidHTTPStatus(httpRes, errorBodyLimit)
	}
	return httpRes, release, nil
}

func marshalToReader[T any](value *T) (*bytes.Reader, error) {
	if value == nil {
		return nil, ErrNilValue
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("%q: %w", err.Error(), ErrMarshalValue)
	}
	return bytes.NewReader(data), nil
}

func buildReader[T Req](request *T) (*bytes.Reader, error) {
	if request == nil {
		return nil, nil
	}
	if reqURL, ok := any(request).(interface {
		URLValues() interface{ Encode() string }
	}); ok {
		return bytes.NewReader([]byte(reqURL.URLValues().Encode())), nil
	}
	return marshalToReader[T](request)
}
