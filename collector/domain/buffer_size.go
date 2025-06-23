package domain

import "errors"

// BufferSize represents a validated buffer size in bytes for I/O operations.
// It ensures that buffer sizes are positive values.
type BufferSize uint32

// NewBufferSize creates a new BufferSize instance.
func NewBufferSize(size int) (BufferSize, error) {
	if size <= 0 {
		return 0, errors.New("buffer size must be greater than 0")
	}

	return BufferSize(size), nil
}
