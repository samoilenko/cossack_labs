package domain

import (
	"errors"
	"time"
)

// RateLimitError represents an error that occurs when a rate limit is exceeded.
// This error type provides additional context about the rate limiting violation,
// including timing information that can be used for retry logic or monitoring.
type RateLimitError struct {
	Delay   time.Duration
	Message string
}

// Error implements the error interface, returning the error message.
func (e *RateLimitError) Error() string {
	return e.Message
}

// ErrValidation is a sentinel error used to indicate validation failures.
// This error should be wrapped with additional context using fmt.Errorf
// to provide specific details about what validation failed.
var ErrValidation = errors.New("validation failed")
