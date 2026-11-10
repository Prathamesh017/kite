package core

import (
	"context"
	"errors"
	"math/rand/v2"
	"os"
	"strconv"
	"time"
)

// retrySettings is how many times a failed request is sent again, and how long to wait.
// The wait starts at initial, doubles each retry, and never goes above max.
type retrySettings struct {
	count   int
	initial time.Duration
	max     time.Duration
}

// retryFromEnv reads MAX_RETRIES, RETRY_INITIAL_DELAY, and RETRY_MAX_DELAY.
// A missing or unreadable value falls back to 3, 500ms, and 8s.
func retryFromEnv() retrySettings {
	r := retrySettings{count: 3, initial: 500 * time.Millisecond, max: 8 * time.Second}
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
func backoff(attempt int, settings retrySettings) time.Duration {
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

// retryable reports whether an error says the same request might succeed if sent
// again. The LLM adapter decides which of its errors say so, by having a
// Retryable method.
func retryable(err error) bool {
	var r interface{ Retryable() bool }
	return errors.As(err, &r) && r.Retryable()
}

// retryLater decides whether a failed request is sent again. If so, the wait is
// remembered in r and the step reports Retrying. The run stays as it is, so the
// next step waits and asks again. It does nothing once the run's context is done,
// because cancel and timeouts are for the caller to handle.
func retryLater(ctx context.Context, r *run, err error) (Result, bool) {
	settings := retryFromEnv()
	if ctx.Err() != nil || !retryable(err) || r.retries >= settings.count {
		return Result{}, false
	}
	r.retryWait = randomUpTo(backoff(r.retries, settings))
	r.retries++
	return Result{Kind: Retrying}, true
}

// waitToRetry waits out the time that retryLater remembered, if any.
func waitToRetry(ctx context.Context, r *run) error {
	wait := r.retryWait
	r.retryWait = 0
	if wait <= 0 {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(wait):
		return nil
	}
}
