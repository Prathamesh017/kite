package llm

import (
	"errors"
	"fmt"
)

// APIError is a failed call to the model. Status is 0 when no response
// arrived (timeout, dropped connection, offline). Message is what Gemini
// sent back, and may be empty.
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Status == 0 {
		return "no response from the model"
	}
	return fmt.Sprintf("model returned %d", e.Status)
}

// Retryable reports whether the same request might succeed if sent again.
// 429 retries for both the per-minute limit and a used-up quota, for now.
func Retryable(err error) bool {
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
