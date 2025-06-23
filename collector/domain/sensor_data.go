package domain

import (
	"time"
)

// SensorData represents a single measurement from a sensor device.
type SensorData struct {
	Timestamp  time.Time
	SensorName string
	Value      int
}
