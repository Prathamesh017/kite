package llm

import (
	"math/rand/v2"
	"os"
	"strconv"
	"time"
)

// Retry is how many times a failed call is sent again, and how long to wait.
// The wait starts at Initial, doubles each retry, and never goes above Max.
type Retry struct {
	Count   int
	Initial time.Duration
	Max     time.Duration
}

// RetryFromEnv reads MAX_RETRIES, RETRY_INITIAL_DELAY, and RETRY_MAX_DELAY.
// A missing or unreadable value falls back to 3, 500ms, and 8s.
func RetryConfigFromEnv() Retry {
	r := Retry{Count: 3, Initial: 500 * time.Millisecond, Max: 8 * time.Second}
	if n, err := strconv.Atoi(os.Getenv("MAX_RETRIES")); err == nil && n >= 0 {
		r.Count = n
	}
	if d, err := time.ParseDuration(os.Getenv("RETRY_INITIAL_DELAY")); err == nil && d > 0 {
		r.Initial = d
	}
	if d, err := time.ParseDuration(os.Getenv("RETRY_MAX_DELAY")); err == nil && d > 0 {
		r.Max = d
	}
	return r
}

// Backoff is the cap for this attempt: Initial, doubled each attempt, never above Max.
func Backoff(attempt int, settings Retry) time.Duration {
	d := settings.Initial
	for i := 0; i < attempt && d < settings.Max; i++ {
		d *= 2
	}
	return min(d, settings.Max)
}

// RandomUpTo picks a wait between zero and d, so retries do not all land at once.
func RandomUpTo(d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	return rand.N(d + 1)
}
