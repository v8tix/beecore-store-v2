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
	Err      string
	Envelope map[string]any
)

func (r Err) Error() string {
	return string(r)
}

var (
	ErrBadlyJSON      = Err("badly-formed JSON in the body")
	ErrBadJSONType    = Err("incorrect JSON type in the body")
	ErrEmptyBody      = Err("body must not be empty")
	ErrBodyUnknownKey = Err("unknown key in the body")
	ErrBodySizeLimit  = Err("body size limit exceeded")
	ErrBodyValue      = Err("body must contain a single JSON value")
)

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
			return err
		}
	}

	err = dec.Decode(&struct{}{})
	if !errors.Is(err, io.EOF) {
		return ErrBodyValue
	}

	return nil
}
