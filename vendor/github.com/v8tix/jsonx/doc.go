// Package jsonx provides a generic, strongly-typed JSON decoder with rich,
// actionable error messages. It wraps encoding/json and surfaces line/column
// positions, context snippets, and sentinel errors that callers can test with
// errors.Is.
package jsonx
