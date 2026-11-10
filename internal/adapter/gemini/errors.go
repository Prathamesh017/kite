package gemini

import "fmt"

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

// Retryable reports whether the same request might succeed if sent again. The
// core decides whether and when to send it again.
// 429 retries for both the per-minute limit and a used-up quota, for now.
func (e *APIError) Retryable() bool {
	switch e.Status {
	case 429, 500, 503, 504:
		return true
	}
	return false
}
