package domain

import (
	"errors"
	"time"
)

// FlushInterval represents the time duration between buffer flush operations.
// This type ensures that only valid, positive durations are used for buffer
type FlushInterval time.Duration

// NewFlushInterval creates a new FlushInterval with validation to ensure
// the interval is positive and suitable for buffer operations.
func NewFlushInterval(val time.Duration) (FlushInterval, error) {
	if val <= 0 {
		return 0, errors.New("buffer flush interval must be greater than 0")
	}
	return FlushInterval(val), nil
}
