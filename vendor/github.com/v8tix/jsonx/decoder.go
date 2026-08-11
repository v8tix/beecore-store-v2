package jsonx

import (
	"bytes"
	"errors"
	"io"

	"encoding/json"
)

// decoder[T] is a configurable JSON decoder built with the Decoder constructor.
//
// It is immutable — every method that changes behaviour returns a new copy,
// so the same base decoder can be safely reused across calls.
//
// Default behaviour (mirrors ReadJSONAs):
//   - Unknown fields are rejected  (strict)
//   - Trailing content is rejected (single-value)
//   - Empty body is rejected
//
// Call Lenient() to allow unknown fields when you only need a subset of the
// JSON object (e.g. partial struct parsing in tests or schema introspection).
type decoder[T any] struct {
	allowUnknown bool
}

// Decoder returns a new strict decoder for type T.
// Chain Lenient() to relax unknown-field rejection before calling From / FromBytes.
//
// Example — strict (equivalent to ReadJSONAs):
//
//	val, err := jsonx.Decoder[MyType]().From(r)
//	val, err := jsonx.Decoder[MyType]().FromBytes(data)
//
// Example — lenient (partial struct, test helper, schema introspection):
//
//	val, err := jsonx.Decoder[MyType]().Lenient().From(r)
//	val, err := jsonx.Decoder[MyType]().Lenient().FromBytes(data)
func Decoder[T any]() decoder[T] { return decoder[T]{} }

// Lenient returns a copy of the decoder that allows unknown JSON fields.
// Use this when the target struct intentionally covers only a subset of the
// JSON object — for example when parsing a third-party schema or inspecting
// a single field from a large response body.
func (d decoder[T]) Lenient() decoder[T] {
	d.allowUnknown = true
	return d
}

// From decodes r into T and returns the typed value.
//
// The body is buffered once in memory to enable rich error context (line/col,
// context snippet). For very large payloads wrap r with http.MaxBytesReader
// before calling From; exceeding the limit returns ErrBodySizeLimit.
//
// Unless Lenient() was called:
//   - Unknown fields  → ErrBodyUnknownKey
//   - Empty body      → ErrEmptyBody
//   - Multiple values → ErrBodyValue
//   - Syntax error    → ErrBadlyJSON  + "line N col M (near …)"
//   - Type mismatch   → ErrBadJSONType + field + expected Go type + JSON token
func (d decoder[T]) From(r io.Reader) (T, error) {
	var zero T

	data, err := io.ReadAll(r)
	if err != nil {
		return zero, err
	}

	var dst T
	dec := json.NewDecoder(bytes.NewReader(data))
	if !d.allowUnknown {
		dec.DisallowUnknownFields()
	}

	if err := dec.Decode(&dst); err != nil {
		return zero, enrichError(data, err)
	}

	// Reject trailing content: a second Decode must return io.EOF.
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return zero, ErrBodyValue
	}

	return dst, nil
}

// FromBytes is a convenience wrapper around From for callers that already hold
// a []byte — avoids the bytes.NewReader boilerplate at the call site.
//
//	val, err := jsonx.Decoder[MyType]().FromBytes(rawJSON)
//	val, err := jsonx.Decoder[MyType]().Lenient().FromBytes(rawJSON)
func (d decoder[T]) FromBytes(b []byte) (T, error) {
	return d.From(bytes.NewReader(b))
}
