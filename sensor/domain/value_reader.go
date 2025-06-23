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
	interval    time.Duration
	maxCapacity uint32
}

func (v *ValueReader) Read(ctx context.Context, sensor Sensor) <-chan *SensorValue {
	valueCH := make(chan *SensorValue, v.maxCapacity)
	ticker := time.NewTicker(v.interval)

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
func NewValueReader(interval time.Duration, maxCapacity uint32, logger Logger) *ValueReader {
	return &ValueReader{
		logger:      logger,
		interval:    interval,
		maxCapacity: maxCapacity,
	}
}
