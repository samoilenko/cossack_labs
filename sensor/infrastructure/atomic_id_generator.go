package infrastructure

import "sync/atomic"

// AtomicIDGenerator provides thread-safe unique ID generation using atomic operations.
type AtomicIDGenerator struct {
	id int64
}

// Generate returns the next unique ID.
func (g *AtomicIDGenerator) Generate() int64 {
	return atomic.AddInt64(&g.id, 1)
}
