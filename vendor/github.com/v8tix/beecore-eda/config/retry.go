package config

import "time"

// RetryConfig defines configuration for retry operations with exponential backoff.
type RetryConfig struct {
	MaxRetries      int           // Maximum number of retry attempts.
	InitialInterval time.Duration // Initial backoff interval.
	MaxInterval     time.Duration // Maximum backoff interval.
	Multiplier      float64       // Backoff multiplier.
	MaxElapsedTime  time.Duration // Maximum total time for all retries.
}

// DefaultRetryConfig returns sensible defaults for retry configuration.
// Use this for normal concurrent operations with 2-10 replicas.
//
// Configuration:
//   - MaxRetries: 20 attempts
//   - InitialInterval: 10ms
//   - MaxInterval: 2s
//   - Multiplier: 2.0 (exponential backoff)
//   - MaxElapsedTime: 30s
//
// Backoff progression: 10ms → 20ms → 40ms → 80ms → 160ms → 320ms → 640ms → 1280ms → 2s (capped)
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:      20,
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     2 * time.Second,
		Multiplier:      2.0,
		MaxElapsedTime:  30 * time.Second,
	}
}

// HighContentionRetryConfig returns configuration for high contention scenarios.
// Use this for flash sales, promotions, or 50+ concurrent replicas.
//
// Configuration:
//   - MaxRetries: 50 attempts
//   - InitialInterval: 5ms (faster initial retry)
//   - MaxInterval: 1s (more frequent retries)
//   - Multiplier: 1.5 (slower growth)
//   - MaxElapsedTime: 60s
//
// Backoff progression: 5ms → 7.5ms → 11.25ms → 16.87ms → 25.31ms → ... → 1s (capped)
func HighContentionRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:      50,
		InitialInterval: 5 * time.Millisecond,
		MaxInterval:     1 * time.Second,
		Multiplier:      1.5,
		MaxElapsedTime:  60 * time.Second,
	}
}

// CustomRetryConfig creates a custom retry configuration.
// Use this when you need fine-grained control over retry behavior.
func CustomRetryConfig(maxRetries int, initialInterval, maxInterval, maxElapsedTime time.Duration, multiplier float64) RetryConfig {
	return RetryConfig{
		MaxRetries:      maxRetries,
		InitialInterval: initialInterval,
		MaxInterval:     maxInterval,
		Multiplier:      multiplier,
		MaxElapsedTime:  maxElapsedTime,
	}
}

// ConservativeRetryConfig returns configuration for operations that should retry cautiously.
// Use this for non-critical operations or when you want to fail fast.
//
// Configuration:
//   - MaxRetries: 5 attempts
//   - InitialInterval: 50ms
//   - MaxInterval: 5s
//   - Multiplier: 2.0
//   - MaxElapsedTime: 10s
func ConservativeRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:      5,
		InitialInterval: 50 * time.Millisecond,
		MaxInterval:     5 * time.Second,
		Multiplier:      2.0,
		MaxElapsedTime:  10 * time.Second,
	}
}

// AggressiveRetryConfig returns configuration for critical operations that must succeed.
// Use this for saga compensations, payment processing, or other critical workflows.
//
// Configuration:
//   - MaxRetries: 100 attempts
//   - InitialInterval: 1ms
//   - MaxInterval: 500ms
//   - Multiplier: 1.2 (very slow growth)
//   - MaxElapsedTime: 120s (2 minutes)
func AggressiveRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:      100,
		InitialInterval: 1 * time.Millisecond,
		MaxInterval:     500 * time.Millisecond,
		Multiplier:      1.2,
		MaxElapsedTime:  120 * time.Second,
	}
}
