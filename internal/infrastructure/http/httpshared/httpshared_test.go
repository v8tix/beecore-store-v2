package httpshared_test

import (
	"errors"
	"testing"
	"time"

	"github.com/v8tix/beecore-eda/config"
	"github.com/v8tix/kawa"

	"github.com/v8tix/beecore-store-v2/internal/infrastructure/http/httpshared"
)

func TestBuildAuthHeader(t *testing.T) {
	got := httpshared.BuildAuthHeader("tok123")

	want := map[string]string{"Authorization": "Bearer tok123"}
	if len(got) != len(want) || got["Authorization"] != want["Authorization"] {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestDecodeHTTPError_NotHTTPError(t *testing.T) {
	networkErr := errors.New("network failure")

	statusCode, message, parseErr, ok := httpshared.DecodeHTTPError(networkErr)

	if ok {
		t.Fatalf("ok = true, want false for a non-HTTP error")
	}
	if statusCode != 0 || message != "" || parseErr != nil {
		t.Fatalf("got (%d, %q, %v), want zero values", statusCode, message, parseErr)
	}
}

func TestDecodeHTTPError_ValidJSONBody(t *testing.T) {
	err := kawa.ErrInvalidHTTPStatus{
		StatusCode: 404,
		Status:     "404 Not Found",
		Body:       []byte(`{"error":"user not found"}`),
	}

	statusCode, message, parseErr, ok := httpshared.DecodeHTTPError(err)

	if !ok {
		t.Fatalf("ok = false, want true for an HTTP status error")
	}
	if parseErr != nil {
		t.Fatalf("unexpected parseErr: %v", parseErr)
	}
	if statusCode != 404 {
		t.Fatalf("got statusCode %d, want 404", statusCode)
	}
	if message != "user not found" {
		t.Fatalf("got message %q, want %q", message, "user not found")
	}
}

// TestDecodeHTTPError_UnparseableBody proves statusCode is still populated
// even when the body isn't valid ErrMessage JSON — callers that need the
// status code on a parse failure don't have to re-decode err themselves.
func TestDecodeHTTPError_UnparseableBody(t *testing.T) {
	err := kawa.ErrInvalidHTTPStatus{
		StatusCode: 500,
		Status:     "500 Internal Server Error",
		Body:       []byte("not json"),
	}

	statusCode, message, parseErr, ok := httpshared.DecodeHTTPError(err)

	if !ok {
		t.Fatalf("ok = false, want true for an HTTP status error")
	}
	if parseErr == nil {
		t.Fatalf("parseErr = nil, want a JSON unmarshal error")
	}
	if statusCode != 500 {
		t.Fatalf("got statusCode %d, want 500", statusCode)
	}
	if message != "" {
		t.Fatalf("got message %q, want empty", message)
	}
}

// TestTranslateHTTPError_ValidJSONBody proves the reconciled behavior uses
// the parsed error message (not the raw body) when the response body is
// valid ErrMessage JSON.
func TestTranslateHTTPError_ValidJSONBody(t *testing.T) {
	err := kawa.ErrInvalidHTTPStatus{
		StatusCode: 400,
		Status:     "400 Bad Request",
		Body:       []byte(`{"error":"bad input"}`),
	}

	got := httpshared.TranslateHTTPError(err)

	want := "downstream request failed with status 400: bad input"
	if got == nil || got.Error() != want {
		t.Fatalf("got %v, want %q", got, want)
	}
}

// TestTranslateHTTPError_UnparseableBody proves the reconciled behavior:
// the status code is always preserved, even when the response body isn't
// valid ErrMessage JSON — falling back to the raw body instead of silently
// discarding the status code and body (the divergent, debugging-hostile
// behavior addressremote's and basketremote's prior translateHTTPError had).
func TestTranslateHTTPError_UnparseableBody(t *testing.T) {
	err := kawa.ErrInvalidHTTPStatus{
		StatusCode: 500,
		Status:     "500 Internal Server Error",
		Body:       []byte("not json"),
	}

	got := httpshared.TranslateHTTPError(err)

	want := "downstream request failed with status 500: not json"
	if got == nil || got.Error() != want {
		t.Fatalf("got %v, want %q", got, want)
	}
}

func TestTranslateHTTPError_NonHTTPError(t *testing.T) {
	networkErr := errors.New("network failure")

	got := httpshared.TranslateHTTPError(networkErr)

	if !errors.Is(got, networkErr) {
		t.Fatalf("got %v, want the original error unchanged", got)
	}
}

func TestDeadline_DefaultsWhenUnset(t *testing.T) {
	got := httpshared.Deadline(&config.Cfg{})

	if got != time.Second {
		t.Fatalf("got %v, want the built-in fast-profile default of 1s", got)
	}
}

func TestRetryPolicy_ReturnsNonNilPolicy(t *testing.T) {
	got := httpshared.RetryPolicy(&config.Cfg{})

	if got == nil {
		t.Fatalf("got nil RetryPolicy")
	}
}
