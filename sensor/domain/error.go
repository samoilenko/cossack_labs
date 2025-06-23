package domain

import (
	"errors"
	"time"
)

// RateLimitError represents an error that occurs when rate limiting is enforced.
// It includes the delay duration that must be waited before retrying.
type RateLimitError struct {
	Delay   time.Duration
	Message string
}

// Error returns the error message, implementing the error interface.
func (e *RateLimitError) Error() string {
	return e.Message
}

// ErrTransportNotReady indicates that the transport layer is not ready to send data.
var ErrTransportNotReady = errors.New("transport is not ready")
