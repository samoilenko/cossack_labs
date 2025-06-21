// Package domain provides core business logic for sensor data processing.
package domain

import (
	"context"
	"time"
)

// Sensor is a data source of values
type Sensor interface {
	GetValue() int
}

// ValueReader reads values from a sensor in a given time period
type ValueReader struct {
	maxCapacity uint32
	rate        Rate
}

func (v *ValueReader) Read(ctx context.Context, sensor Sensor) <-chan int {
	valueCH := make(chan int, v.maxCapacity)
	delay := time.Second / time.Duration(v.rate)
	ticker := time.NewTicker(delay)

	go func() {
		for {
			select {
			case <-ctx.Done():
				close(valueCH)
				ticker.Stop()
				return
			case <-ticker.C:
				valueCH <- sensor.GetValue()
			}
		}
	}()

	return valueCH
}

// NewValueReader creates a ValueReader with the given rate and buffer size
func NewValueReader(rate Rate, maxCapacity uint32) (*ValueReader, error) {
	return &ValueReader{
		rate:        rate,
		maxCapacity: maxCapacity,
	}, nil
}
