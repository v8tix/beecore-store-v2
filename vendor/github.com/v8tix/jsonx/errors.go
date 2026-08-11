package jsonx

// jsonxErr is an unexported string-based error type so sentinel values are
// comparable with == and usable in errors.Is chains without allocation.
type jsonxErr string

func (e jsonxErr) Error() string { return string(e) }

// Sentinel errors returned by ReadJSONAs. Use errors.Is to test for them.
var (
	// ErrBadlyJSON is returned when the input is not valid JSON
	// (syntax error or unexpected end of input).
	ErrBadlyJSON = jsonxErr("badly-formed JSON in the body")

	// ErrBadJSONType is returned when a JSON value cannot be decoded into the
	// target Go type (type mismatch).
	ErrBadJSONType = jsonxErr("incorrect JSON type in the body")

	// ErrEmptyBody is returned when the reader contains no bytes.
	ErrEmptyBody = jsonxErr("body must not be empty")

	// ErrBodyUnknownKey is returned when the JSON object contains a key that
	// does not map to any field in the target struct.
	ErrBodyUnknownKey = jsonxErr("unknown key in the body")

	// ErrBodySizeLimit is returned when an http.MaxBytesReader limit is hit
	// while reading the body.
	ErrBodySizeLimit = jsonxErr("body size limit exceeded")

	// ErrBodyValue is returned when the input contains more than one top-level
	// JSON value (e.g., two objects back-to-back).
	ErrBodyValue = jsonxErr("body must contain a single JSON value")
)
