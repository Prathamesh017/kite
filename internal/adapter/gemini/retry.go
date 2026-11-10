package gemini

import (
	"math/rand/v2"
	"os"
	"strconv"
	"time"
)

// retry is how many times a failed call is sent again, and how long to wait.
// The wait starts at initial, doubles each retry, and never goes above max.
type retry struct {
	count   int
	initial time.Duration
	max     time.Duration
}

// retryFromEnv reads MAX_RETRIES, RETRY_INITIAL_DELAY, and RETRY_MAX_DELAY.
// A missing or unreadable value falls back to 3, 500ms, and 8s.
func retryFromEnv() retry {
	r := retry{count: 3, initial: 500 * time.Millisecond, max: 8 * time.Second}
	if n, err := strconv.Atoi(os.Getenv("MAX_RETRIES")); err == nil && n >= 0 {
		r.count = n
	}
	if d, err := time.ParseDuration(os.Getenv("RETRY_INITIAL_DELAY")); err == nil && d > 0 {
		r.initial = d
	}
	if d, err := time.ParseDuration(os.Getenv("RETRY_MAX_DELAY")); err == nil && d > 0 {
		r.max = d
	}
	return r
}

// backoff is the cap for this attempt: initial, doubled each attempt, never above max.
func backoff(attempt int, settings retry) time.Duration {
	d := settings.initial
	for i := 0; i < attempt && d < settings.max; i++ {
		d *= 2
	}
	return min(d, settings.max)
}

// randomUpTo picks a wait between zero and d, so retries do not all land at once.
func randomUpTo(d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	return rand.N(d + 1)
}

// requestTimeout reads REQUEST_TIMEOUT. A missing or unreadable value falls back to 60s.
func requestTimeout() time.Duration {
	if d, err := time.ParseDuration(os.Getenv("REQUEST_TIMEOUT")); err == nil && d > 0 {
		return d
	}
	return 60 * time.Second
}
