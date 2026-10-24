// Package utils holds small, shared helpers (JSON decoding, pagination, request parsing)
// used across the HTTP layer of several microservices.
package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type (
	// JSONError is a simple string-backed error type for static, comparable sentinel errors.
	JSONError string
	// Envelope is a generic JSON response wrapper keyed by field name.
	Envelope map[string]any
)

func (r JSONError) Error() string {
	return string(r)
}

// Sentinel errors returned by ReadJSON for the different ways a request body can be malformed.
var (
	ErrBadlyJSON      = JSONError("badly-formed JSON in the body")
	ErrBadJSONType    = JSONError("incorrect JSON type in the body")
	ErrEmptyBody      = JSONError("body must not be empty")
	ErrBodyUnknownKey = JSONError("unknown key in the body")
	ErrBodySizeLimit  = JSONError("body size limit exceeded")
	ErrBodyValue      = JSONError("body must contain a single JSON value")
)

// ReadJSON decodes a single JSON value from body into dst, rejecting unknown fields and
// translating low-level encoding/json errors into one of the sentinel errors above.
//
//nolint:cyclop // one branch per distinct encoding/json failure mode; splitting would obscure the 1:1 mapping.
func ReadJSON(body io.Reader, dst any) error {
	dec := json.NewDecoder(body)
	dec.DisallowUnknownFields()

	err := dec.Decode(dst)
	if err != nil {
		var syntaxError *json.SyntaxError

		var unmarshalTypeError *json.UnmarshalTypeError

		var invalidUnmarshalError *json.InvalidUnmarshalError

		var maxBytesError *http.MaxBytesError

		switch {
		case errors.As(err, &syntaxError):
			return fmt.Errorf("%w: at character %d", ErrBadlyJSON, syntaxError.Offset)
		case errors.Is(err, io.ErrUnexpectedEOF):
			return ErrBadlyJSON
		case errors.As(err, &unmarshalTypeError):
			if unmarshalTypeError.Field != "" {
				return fmt.Errorf("%w for field %q", ErrBadJSONType, unmarshalTypeError.Field)
			}

			return fmt.Errorf("%w at character %d", ErrBadJSONType, unmarshalTypeError.Offset)
		case errors.Is(err, io.EOF):
			return ErrEmptyBody
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")

			return fmt.Errorf("%w %s", ErrBodyUnknownKey, fieldName)
		case errors.As(err, &maxBytesError):
			return fmt.Errorf("%w. Max size is %d bytes", ErrBodySizeLimit, maxBytesError.Limit)
		case errors.As(err, &invalidUnmarshalError):
			panic(err)
		default:
			return fmt.Errorf("decode request body: %w", err)
		}
	}

	err = dec.Decode(&struct{}{})
	if !errors.Is(err, io.EOF) {
		return ErrBodyValue
	}

	return nil
}
