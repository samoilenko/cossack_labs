// Package infrastructure provides concrete implementation of sensor domain entities.
package infrastructure

import "math/rand/v2"

// DummySensor is a sensor implementation that generates random integer values
type DummySensor struct {
}

// GetValue return a random integer value between 0 and 99
func (d DummySensor) GetValue() (int32, error) {
	return int32(rand.IntN(100)), nil
}
