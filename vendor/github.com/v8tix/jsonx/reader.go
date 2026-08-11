package jsonx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const snippetWindow = 20

// ReadJSONAs decodes r into T and returns the typed value.
//
// This is a strict convenience wrapper around Decoder[T]().From(r).
// For configurable behaviour (e.g. allowing unknown fields or decoding from
// []byte) use the Decoder builder directly:
//
//	jsonx.Decoder[T]().From(r)
//	jsonx.Decoder[T]().FromBytes(data)
//	jsonx.Decoder[T]().Lenient().From(r)
//
// Strict semantics:
//   - Unknown fields  → ErrBodyUnknownKey
//   - Empty body      → ErrEmptyBody
//   - Multiple values → ErrBodyValue
//   - Syntax error    → ErrBadlyJSON  + "line N col M (near …)"
//   - Type mismatch   → ErrBadJSONType + field + expected Go type + JSON token
//   - Size limit      → ErrBodySizeLimit (only when http.MaxBytesReader is used)
//
// All returned errors wrap the relevant sentinel so errors.Is works:
//
//	val, err := jsonx.ReadJSONAs[MyType](r)
//	if errors.Is(err, jsonx.ErrBadlyJSON) { ... }
func ReadJSONAs[T any](r io.Reader) (T, error) {
	return Decoder[T]().From(r)
}

// enrichError converts a raw json decode error into a richly-annotated error
// that wraps the appropriate jsonx sentinel.
func enrichError(data []byte, err error) error {
	var (
		syntaxError        *json.SyntaxError
		unmarshalTypeError *json.UnmarshalTypeError
		invalidUnmarshal   *json.InvalidUnmarshalError
		maxBytesErr        *http.MaxBytesError
	)

	switch {
	case errors.As(err, &syntaxError):
		// SyntaxError.Offset is 1-based (byte position after the bad character).
		offset := syntaxError.Offset - 1
		line, col := offsetToLineCol(data, offset)
		snippet := contextSnippet(data, offset)
		return fmt.Errorf("%w: line %d col %d (near %s)", ErrBadlyJSON, line, col, snippet)

	case errors.Is(err, io.ErrUnexpectedEOF):
		// Truncated JSON — no precise offset; point to end of input.
		offset := int64(len(data)) - 1
		if offset < 0 {
			offset = 0
		}
		line, col := offsetToLineCol(data, offset)
		return fmt.Errorf("%w: unexpected end of JSON at line %d col %d", ErrBadlyJSON, line, col)

	case errors.As(err, &unmarshalTypeError):
		// UnmarshalTypeError.Offset is 1-based.
		offset := unmarshalTypeError.Offset - 1
		line, col := offsetToLineCol(data, offset)
		if unmarshalTypeError.Field != "" {
			return fmt.Errorf("%w for field %q (line %d col %d): expected %s, got JSON %s",
				ErrBadJSONType,
				unmarshalTypeError.Field,
				line, col,
				unmarshalTypeError.Type,
				unmarshalTypeError.Value,
			)
		}
		return fmt.Errorf("%w (line %d col %d): expected %s, got JSON %s",
			ErrBadJSONType,
			line, col,
			unmarshalTypeError.Type,
			unmarshalTypeError.Value,
		)

	case errors.Is(err, io.EOF):
		return ErrEmptyBody

	case strings.HasPrefix(err.Error(), "json: unknown field "):
		fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
		return fmt.Errorf("%w %s", ErrBodyUnknownKey, fieldName)

	case errors.As(err, &maxBytesErr):
		return fmt.Errorf("%w. Max size is %d bytes", ErrBodySizeLimit, maxBytesErr.Limit)

	case errors.As(err, &invalidUnmarshal):
		// Programmer error: dst is not a pointer.
		// ReadJSONAs prevents this at compile time via generics, but guard anyway.
		panic(err)

	default:
		return err
	}
}

// offsetToLineCol converts a 0-based byte offset into 1-based (line, col).
// Column counts bytes, not Unicode runes — consistent with json package offsets.
func offsetToLineCol(data []byte, offset int64) (line, col int) {
	line = 1
	col = 1
	for i := int64(0); i < offset && i < int64(len(data)); i++ {
		if data[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return line, col
}

// contextSnippet returns a quoted, trimmed window of bytes around offset
// suitable for embedding in a human-readable error message.
func contextSnippet(data []byte, offset int64) string {
	start := offset - snippetWindow
	if start < 0 {
		start = 0
	}
	end := offset + snippetWindow
	if end > int64(len(data)) {
		end = int64(len(data))
	}
	return fmt.Sprintf("%q", strings.TrimSpace(string(data[start:end])))
}
