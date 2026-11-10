package gemini

import (
	"os"
	"time"
)

// requestTimeout reads REQUEST_TIMEOUT. A missing or unreadable value falls back to 60s.
func requestTimeout() time.Duration {
	if d, err := time.ParseDuration(os.Getenv("REQUEST_TIMEOUT")); err == nil && d > 0 {
		return d
	}
	return 60 * time.Second
}
