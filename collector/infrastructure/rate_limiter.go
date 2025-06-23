package infrastructure

import (
	"sync"
	"time"

	collectorDomain "github.com/samoilenko/cossack_labs/collector/domain"
	"google.golang.org/protobuf/proto"
)

// RateLimiter implements a fixed window rate limiter for protobuf messages.
// It limits the total size of messages processed within a 1-second window.
type RateLimiter[K proto.Message] struct {
	last       time.Time
	mu         sync.Mutex
	timeWindow time.Duration
	maxLimit   collectorDomain.RateLimit
	limit      int
}

// Apply processes a message through the rate limiter, checking if it exceeds the configured rate limit.
//
// The rate limiter uses a fixed window approach:
// - If more than 1 second has elapsed since the last reset, it starts a new window
// - Otherwise, it accumulates the message size and checks against the limit
// - Messages exceeding the limit are rejected with a RateLimitError
func (r *RateLimiter[K]) Apply(msg K) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	elapsed := now.Sub(r.last)
	size := proto.Size(msg)
	if elapsed >= r.timeWindow {
		r.last = now
		r.limit = size

		return nil
	}

	r.limit += size
	if r.limit > int(r.maxLimit) {
		return &collectorDomain.RateLimitError{
			Message: "rate limit exceeded",
			Delay:   elapsed,
		}
	}

	return nil
}

// NewRateLimiter creates a new rate limiter instance with the specified maximum limit.
func NewRateLimiter[K proto.Message](maxLimit collectorDomain.RateLimit, timeWindow time.Duration) *RateLimiter[K] {
	return &RateLimiter[K]{
		maxLimit:   maxLimit,
		timeWindow: timeWindow,
	}
}
