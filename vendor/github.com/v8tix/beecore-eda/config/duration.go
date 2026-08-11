package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ErrInvalidDurationFormat is returned when a Duration's JSON value can't be parsed.
var ErrInvalidDurationFormat = errors.New("invalid duration format")

var humanDurationPattern = regexp.MustCompile(`(?i)^(\d+)\s*(millisecond|milliseconds|second|seconds|minute|minutes|hour|hours|day|days)$`)

// Duration is a JSON-friendly time.Duration. It accepts either a human-readable
// "<N> <unit>" string (e.g. "72 hours", "30 days", "250 milliseconds") or a
// plain Go duration string (e.g. "24h", "15m30s") when unmarshaling, and
// marshals back out as a Go duration string.
//
// Example config:
//
//	{"activation_token_expiration": "72 hours"}
type Duration struct {
	d time.Duration
}

// NewDuration wraps a time.Duration as a Duration, e.g. for default values.
func NewDuration(d time.Duration) Duration {
	return Duration{d: d}
}

// Duration returns the underlying time.Duration.
func (d Duration) Duration() time.Duration {
	return d.d
}

// String returns the duration in Go's standard duration format.
func (d Duration) String() string {
	return d.d.String()
}

// UnmarshalJSON implements json.Unmarshaler.
func (d *Duration) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidDurationFormat, err)
	}

	parsed, err := parseDurationString(s)
	if err != nil {
		return err
	}

	d.d = parsed

	return nil
}

// MarshalJSON implements json.Marshaler.
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.d.String())
}

func parseDurationString(s string) (time.Duration, error) {
	trimmed := strings.TrimSpace(s)

	if match := humanDurationPattern.FindStringSubmatch(trimmed); match != nil {
		n, err := strconv.Atoi(match[1])
		if err != nil {
			return 0, fmt.Errorf("%w: %s", ErrInvalidDurationFormat, s)
		}

		return time.Duration(n) * unitDuration(strings.ToLower(match[2])), nil
	}

	parsed, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, fmt.Errorf("%w: %s", ErrInvalidDurationFormat, s)
	}

	if parsed < 0 {
		return 0, fmt.Errorf("%w: negative duration not allowed: %s", ErrInvalidDurationFormat, s)
	}

	return parsed, nil
}

func unitDuration(unit string) time.Duration {
	switch {
	case strings.HasPrefix(unit, "millisecond"):
		return time.Millisecond
	case strings.HasPrefix(unit, "second"):
		return time.Second
	case strings.HasPrefix(unit, "minute"):
		return time.Minute
	case strings.HasPrefix(unit, "hour"):
		return time.Hour
	default: // "day"/"days" — the regex only matches these seven unit spellings
		return 24 * time.Hour
	}
}
