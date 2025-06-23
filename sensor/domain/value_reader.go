// Package domain provides core business logic for sensor data processing.
// A sensor node - application which emits telemetry data
package domain

import (
	"context"
	"time"
)

// Sensor is a data source of values
type Sensor interface {
	GetValue() (int32, error)
}

// SensorValue provides data received from a sensor
type SensorValue struct {
	Timestamp time.Time
	Value     int32
}

// ValueReader reads values from a sensor in a given time period
type ValueReader struct {
	logger      Logger
	maxCapacity uint32
	rate        Rate
}

func (v *ValueReader) Read(ctx context.Context, sensor Sensor) <-chan *SensorValue {
	valueCH := make(chan *SensorValue, v.maxCapacity)
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
				value, err := sensor.GetValue()
				if err != nil {
					v.logger.Error("error on getting value: %s", err.Error())
					continue
				}
				valueCH <- &SensorValue{
					Value:     value,
					Timestamp: time.Now(),
				}
			}
		}
	}()

	return valueCH
}

// NewValueReader creates a ValueReader with the given rate and buffer size
func NewValueReader(rate Rate, maxCapacity uint32, logger Logger) *ValueReader {
	return &ValueReader{
		logger:      logger,
		rate:        rate,
		maxCapacity: maxCapacity,
	}
}
