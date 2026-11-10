package gemini

import (
	"errors"
	"fmt"
)

// APIError is a failed call to the model that got a response. Message is what
// Gemini sent back, and may be empty.
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("model returned %d: %s", e.Status, e.Message)
	}
	return fmt.Sprintf("model returned %d", e.Status)
}

// retryable reports whether the same request might succeed if sent again.
// 429 retries for both the per-minute limit and a used-up quota, for now.
func retryable(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	switch apiErr.Status {
	case 429, 500, 503, 504:
		return true
	}
	return false
}
