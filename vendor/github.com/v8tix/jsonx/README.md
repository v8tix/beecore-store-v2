# jsonx

A tiny Go package that makes JSON decoding **actually tell you what went wrong**.

```
go get github.com/v8tix/jsonx
```

---

## Why?

The standard library's `encoding/json` is great, but its errors can be... terse.

```
invalid character '}' looking for beginning of value
```

`jsonx` gives you this instead:

```
badly-formed JSON in the body: line 3 col 11 (near "\"age\": }")
```

Every error wraps a typed sentinel so you can branch on it with `errors.Is`,
and every message includes the line, column, and a context snippet so you can
fix the problem without guessing.

---

## Quick start

```go
import "github.com/v8tix/jsonx"

type Order struct {
    ID    string `json:"id"`
    Total int    `json:"total"`
}

// From an io.Reader (e.g. HTTP response body)
func parseOrder(r io.Reader) (Order, error) {
    return jsonx.ReadJSONAs[Order](r)
}

// From a []byte — no bytes.NewReader boilerplate needed
func parseOrderBytes(data []byte) (Order, error) {
    return jsonx.Decoder[Order]().FromBytes(data)
}
```

---

## Two APIs, one rule set

### `ReadJSONAs` — strict, from `io.Reader`

```go
val, err := jsonx.ReadJSONAs[Order](r)
```

The original API. Always strict: unknown fields and trailing content are
rejected. Use this for HTTP handlers and any case where you own the full schema.

### `Decoder` builder — configurable

```go
// strict (equivalent to ReadJSONAs)
val, err := jsonx.Decoder[Order]().From(r)

// from []byte — eliminates bytes.NewReader at every call site
val, err := jsonx.Decoder[Order]().FromBytes(data)

// lenient — unknown fields are allowed (partial struct, schema introspection, tests)
val, err := jsonx.Decoder[Order]().Lenient().From(r)
val, err := jsonx.Decoder[Order]().Lenient().FromBytes(data)
```

The builder is **immutable** — `Lenient()` returns a new copy and never mutates
the original, so the same base decoder can be safely reused:

```go
strict  := jsonx.Decoder[Order]()
lenient := strict.Lenient() // strict is unchanged
```

#### When to use `Lenient()`

| Situation | Why lenient is needed |
|---|---|
| Partial structs in tests | You only declare the fields you want to assert |
| Third-party API responses | The schema may add fields you don't control |
| Schema / metadata introspection | You intentionally parse a subset of a larger object |

```go
// Test helper — only inspect the "role" field of a large message object
type roleOnly struct {
    Role string `json:"role"`
}
msg, err := jsonx.Decoder[roleOnly]().Lenient().FromBytes(rawMessage)

// MCP schema introspection — extract properties from a larger JSON Schema object
type schemaProps struct {
    Properties map[string]any `json:"properties"`
    Required   []string       `json:"required"`
}
schema, err := jsonx.Decoder[schemaProps]().Lenient().FromBytes(rawSchema)
```

---

## What it checks

| Situation | Sentinel | Message includes |
|-----------|----------|-----------------|
| Empty body | `ErrEmptyBody` | — |
| Syntax error | `ErrBadlyJSON` | line, col, context snippet |
| Truncated JSON | `ErrBadlyJSON` | line, col at cut point |
| Wrong type for a field | `ErrBadJSONType` | field name, expected Go type, JSON token |
| Unknown field in object | `ErrBodyUnknownKey` | the offending key name |
| More than one top-level value | `ErrBodyValue` | — |
| Body too large | `ErrBodySizeLimit` | the configured byte limit |

> `ErrBodyUnknownKey` is only raised in **strict** mode. `Lenient()` suppresses it.
> All other sentinels apply in both modes.

All errors wrap their sentinel, so `errors.Is` always works — even through
additional wrapping layers:

```go
val, err := jsonx.ReadJSONAs[Order](r)
switch {
case err == nil:
    // happy path
case errors.Is(err, jsonx.ErrBadlyJSON):
    http.Error(w, "malformed JSON: "+err.Error(), http.StatusBadRequest)
case errors.Is(err, jsonx.ErrBadJSONType):
    http.Error(w, "wrong value type: "+err.Error(), http.StatusUnprocessableEntity)
case errors.Is(err, jsonx.ErrBodyUnknownKey):
    http.Error(w, "unexpected field: "+err.Error(), http.StatusBadRequest)
case errors.Is(err, jsonx.ErrEmptyBody):
    http.Error(w, "request body is required", http.StatusBadRequest)
default:
    http.Error(w, "internal error", http.StatusInternalServerError)
}
```

---

## Body size limit

`ReadJSONAs` and `Decoder.From` buffer the full body in memory. If you need to
cap allocation, wrap the reader with
[`http.MaxBytesReader`](https://pkg.go.dev/net/http#MaxBytesReader) before
passing it in — `jsonx` will translate the limit error into `ErrBodySizeLimit`:

```go
r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB cap
order, err := jsonx.ReadJSONAs[Order](r.Body)
if errors.Is(err, jsonx.ErrBodySizeLimit) {
    http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
}
```

---

## Strict decoding (default)

Both `ReadJSONAs` and `Decoder[T]()` use **strict** semantics by default:

- Unknown fields are rejected (`ErrBodyUnknownKey`) — no silent data loss.
- Trailing content after the first value is rejected (`ErrBodyValue`) — avoids
  accidentally accepting concatenated payloads.

Call `.Lenient()` on the builder to relax unknown-field rejection when needed.

---

## Notes

- Line and column numbers count **bytes**, not Unicode runes — consistent with
  the offsets reported by `encoding/json`.
- The context snippet around a syntax error is 20 bytes, `%q`-formatted.
- `ReadJSONAs` is a one-line wrapper around `Decoder[T]().From(r)` — identical
  behaviour, kept for backwards compatibility.

---

## License

[MIT](./LICENSE) — free to use, modify, and ship in commercial products.
