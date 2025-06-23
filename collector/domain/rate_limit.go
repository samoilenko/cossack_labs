package domain

import "errors"

// RateLimit represents the maximum allowed input flow rate in bytes per second.
// This type ensures that only valid, positive rate limits are configured.
type RateLimit uint32

// NewRateLimit creates a new RateLimit with validation to ensure the limit is positive.
func NewRateLimit(val int) (RateLimit, error) {
	if val <= 0 {
		return 0, errors.New("rate limit must be greater than 0")
	}
	return RateLimit(val), nil
}
