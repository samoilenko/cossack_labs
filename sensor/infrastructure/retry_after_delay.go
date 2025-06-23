package infrastructure

import (
	"sync"
	"time"
)

// RetryAfterDelay provides storage for retry delay duration value.
type RetryAfterDelay struct {
	mu    sync.RWMutex
	value time.Duration
}

// Get returns the current retry delay duration.
func (d *RetryAfterDelay) Get() time.Duration {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.value
}

// Set updates the retry delay duration.
func (d *RetryAfterDelay) Set(delay time.Duration) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.value = delay
}

// Reset clears the retry delay duration
func (d *RetryAfterDelay) Reset() {
	d.Set(0)
}

// NewRetryAfterDelay creates a new RetryAfterDelay instance with zero initial delay.
func NewRetryAfterDelay() *RetryAfterDelay {
	return &RetryAfterDelay{}
}
