package config

import "time"

// CallProfile groups HTTP call tuning knobs for a class of external call: how
// long to wait per attempt, how many times to retry, and how long to wait
// between retries.
// MaxRetries is uint64 to match roku.FetchRx's backoffRetries parameter directly,
// so callers can pass a profile's fields through without a manual conversion.
type CallProfile struct {
	Timeout       Duration `json:"timeout,omitempty"`
	MaxRetries    uint64   `json:"max_retries,omitempty"`
	RetryInterval Duration `json:"retry_interval,omitempty"`
}

func (p CallProfile) resolve(def CallProfile) CallProfile {
	resolved := p
	if resolved.Timeout.Duration() == 0 {
		resolved.Timeout = def.Timeout
	}
	if resolved.MaxRetries == 0 {
		resolved.MaxRetries = def.MaxRetries
	}
	if resolved.RetryInterval.Duration() == 0 {
		resolved.RetryInterval = def.RetryInterval
	}
	return resolved
}

var (
	defaultFastProfile = CallProfile{
		Timeout: NewDuration(time.Second), MaxRetries: 3, RetryInterval: NewDuration(100 * time.Millisecond),
	}
	defaultDefaultProfile = CallProfile{
		Timeout: NewDuration(2 * time.Second), MaxRetries: 3, RetryInterval: NewDuration(250 * time.Millisecond),
	}
	defaultSlowProfile = CallProfile{
		Timeout: NewDuration(10 * time.Second), MaxRetries: 3, RetryInterval: NewDuration(500 * time.Millisecond),
	}
)

// HTTPClient holds named call-tuning profiles for cross-service HTTP calls, so
// callers pick a profile matching how long that kind of call is expected to
// take instead of hardcoding timeout/retry constants per repository. Any field
// left unset in config falls back to that profile's built-in default.
type HTTPClient struct {
	// Fast is for simple, low-latency calls: single-record lookups by ID.
	Fast CallProfile `json:"fast,omitempty"`
	// Default is for calls with no more specific profile.
	Default CallProfile `json:"default,omitempty"`
	// Slow is for calls expected to take noticeably longer, e.g. payment
	// processing or other heavier cross-service operations.
	Slow CallProfile `json:"slow,omitempty"`
}

// GetFast returns the "fast" call profile, falling back to built-in defaults
// for any field not set in config.
func (h HTTPClient) GetFast() CallProfile { return h.Fast.resolve(defaultFastProfile) }

// GetDefault returns the "default" call profile, falling back to built-in
// defaults for any field not set in config.
func (h HTTPClient) GetDefault() CallProfile { return h.Default.resolve(defaultDefaultProfile) }

// GetSlow returns the "slow" call profile, falling back to built-in defaults
// for any field not set in config.
func (h HTTPClient) GetSlow() CallProfile { return h.Slow.resolve(defaultSlowProfile) }
