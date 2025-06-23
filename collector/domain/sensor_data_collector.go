// Package domain contains the core business logic and domain models
// for the sensor data collection system.
package domain

import (
	"bytes"
	"context"
	"fmt"
	"time"
)

// Writer defines the interface for writing sensor data to various destinations.
type Writer interface {
	// IsReady returns true if the writer is ready to accept data.
	// This should be checked before attempting to write data.
	IsReady() bool

	// Write sends the provided data to the destination.
	// Returns an error if the write operation fails.
	Write(data []byte) error

	// Reconnect attempts to re-establish connection to the destination.
	Reconnect(ctx context.Context) error

	// Close cleanly shuts down the writer and releases any resources.
	Close() error
}

// SensorDataCollector manages the collection and writing of sensor data.
// It handles automatic reconnection when write operations fail and provides
// a robust mechanism for processing continuous sensor data streams.
type SensorDataCollector struct {
	logger      Logger
	writer      Writer
	reconnectCh chan bool
}

// Start listens for context cancellation and reconnection triggers.
// This method blocks until the context is cancelled.
func (fc *SensorDataCollector) Start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-fc.reconnectCh:
			if err := fc.writer.Reconnect(ctx); err != nil {
				fc.logger.Error("error on reconnecting: %s", err.Error())
			}
		}
	}
}

// triggerReconnect sends a non-blocking signal to initiate a reconnection attempt.
// If the reconnection channel is full, the trigger is ignored to prevent blocking.
func (fc *SensorDataCollector) triggerReconnect() {
	select {
	case fc.reconnectCh <- true:
	default:
	}
}

// WaitWriter blocks until the writer becomes ready or the context is cancelled.
// It polls the writer's ready state every second if initially not ready.
func (fc *SensorDataCollector) WaitWriter(ctx context.Context) {
	if fc.writer.IsReady() {
		return
	}
	fc.logger.Error("writer is not ready")
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second * 1):
			if fc.writer.IsReady() {
				return
			}
		}
	}
}

// Consume processes sensor data from the provided channel and writes it using the configured writer.
// The method continues processing until the channel is closed.
func (fc *SensorDataCollector) Consume(ctx context.Context, dataChannel <-chan *SensorData) {
	for sensorData := range dataChannel {
		fc.WaitWriter(ctx)

		var buf bytes.Buffer
		fmt.Fprintf(&buf, "name=%s \t value=%d \t time=%s \n",
			sensorData.SensorName, sensorData.Value, sensorData.Timestamp.Format(time.RFC3339),
		)
		logLine := buf.Bytes()

		fc.logger.Info("adding data to writer")
		if err := fc.writer.Write(logLine); err != nil {
			fc.logger.Error("error occurred on adding data to writer: %s", err.Error())

			for {
				fc.triggerReconnect()
				fc.WaitWriter(ctx)
				if err := fc.writer.Write(logLine); err != nil {
					fc.logger.Error("error occurred on adding data to writer: %s", err.Error())
					continue
				}
				break
			}
		}
	}
}

// NewSensorDataCollector creates a new SensorDataCollector with the provided logger and writer.
func NewSensorDataCollector(logger Logger, writer Writer) *SensorDataCollector {
	return &SensorDataCollector{
		logger:      logger,
		reconnectCh: make(chan bool, 1),
		writer:      writer,
	}
}
