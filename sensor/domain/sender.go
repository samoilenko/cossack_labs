package domain

import (
	"context"
	"errors"
	"time"
)

// Transport defines the contract for sending data to external systems.
type Transport interface {
	// Send transfers sensor data.
	Send(ctx context.Context, value int32, timestamp time.Time, sensorName SensorName) error
}

// SensorDataSender handles the transmission of sensor data using a configured transport.
type SensorDataSender struct {
	transport  Transport
	sensorName SensorName
	logger     Logger
}

// Send processes sensor values from the channel and transmits them using the configured transport.
// It handles rate limiting by waiting for the specified delay and retries when transport is not ready.
// The method blocks until the context is cancelled or the input channel is closed.
func (s *SensorDataSender) Send(ctx context.Context, values <-chan *SensorValue) {
	for v := range values {
		// TODO: add timeout
		err := s.transport.Send(ctx, v.Value, v.Timestamp, s.sensorName)
		if err == nil {
			continue
		}

		var rateLimitError *RateLimitError
		if errors.As(err, &rateLimitError) {
			s.logger.Info("data ignored due to rate limit. Next send after %s", rateLimitError.Delay)
			select {
			case <-ctx.Done():
				return
			case <-time.After(rateLimitError.Delay):
				s.logger.Info("delay is finished")
			}
		} else if errors.Is(err, ErrTransportNotReady) {
			s.logger.Error("get transport error: %s", err.Error())
			delay := time.Second * 1
			s.logger.Info("transport is not ready, waiting for %s", delay)
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):

			}
		} else {
			s.logger.Error("error sending data: %s", err.Error())
		}
	}
}

// NewSensorDataSender creates a new SensorDataSender with the specified transport, logger, and sensor name.
func NewSensorDataSender(transport Transport, logger Logger, sensorName SensorName) *SensorDataSender {
	return &SensorDataSender{
		transport:  transport,
		sensorName: sensorName,
		logger:     logger,
	}
}
