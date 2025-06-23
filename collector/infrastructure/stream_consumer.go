// Package infrastructure provides concrete implementations of domain abstractions.
//
// Key components:
//   - StreamConsumer: Handles bidirectional gRPC streams for sensor data
//   - RateLimiter: Implements message rate limiting based on size
//   - SensorDataValidator: Validates incoming sensor data messages
package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"connectrpc.com/connect"

	collectorDomain "github.com/samoilenko/cossack_labs/collector/domain"
	gen "github.com/samoilenko/cossack_labs/pkg/sensorpb/v1"
	durationpb "google.golang.org/protobuf/types/known/durationpb"
)

// StreamConsumer handles bidirectional gRPC streams for sensor data collection.
// It processes incoming sensor data messages, applies interceptors,
// and forwards valid data to a channel for further processing.
type StreamConsumer struct {
	closedLock   sync.RWMutex
	logger       collectorDomain.Logger
	dataConsumer chan *collectorDomain.SensorData
	interceptors *collectorDomain.Interceptors[gen.SensorData]
	closed       bool
}

// GetReadDataChannel returns a read-only channel containing processed sensor data.
func (s *StreamConsumer) GetReadDataChannel() <-chan *collectorDomain.SensorData {
	return s.dataConsumer
}

// Stop shuts down the stream consumer by closing the data channel.
// After calling Stop, no new data will be sent to the data channel.
func (s *StreamConsumer) Stop() {
	s.closedLock.Lock()
	defer s.closedLock.Unlock()
	if s.closed {
		return
	}
	s.closed = true

	close(s.dataConsumer)
}

// send safely transmits sensor data to the data channel.
// This method prevents writing to a closed channel during shutdown.
func (s *StreamConsumer) send(ctx context.Context, sensorData *collectorDomain.SensorData) error {
	s.closedLock.RLock()
	defer s.closedLock.RUnlock()

	if s.closed { // prevents writing to closed channel on shutdown
		return errors.New("consumer is closed")
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case s.dataConsumer <- sensorData:
		return nil
	}
}

// GetStream processes a bidirectional gRPC stream of sensor data.
//
// Message Processing Flow:
// 1. Receives sensor data messages from the client stream
// 2. Applies interceptor chain
// 3. For valid messages: converts to domain model and forwards to data channel
// 4. For invalid messages: generates appropriate error response to client
//
// Error Handling:
// - Rate limit errors: Returns CODE_RESOURCE_EXHAUSTED with retry delay
// - Validation errors: Returns CODE_INVALID_ARGUMENT with error details
// - Internal errors: Returns CODE_INTERNAL (details logged, not exposed to client)
func (s *StreamConsumer) GetStream(ctx context.Context, stream *connect.BidiStream[gen.SensorData, gen.Response]) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		msg, err := stream.Receive()
		if err != nil {
			if err == io.EOF {
				s.logger.Info("Client closed stream")
				return nil
			}
			return err
		}
		if err = s.interceptors.Apply(msg); err == nil {
			s.logger.Info("passing data to consumer. \t correlation Id: %d", msg.CorrelationId)
			sensorData := &collectorDomain.SensorData{
				SensorName: msg.SensorName,
				Value:      int(msg.SensorValue),
				Timestamp:  msg.Timestamp.AsTime(),
			}

			if err := s.send(ctx, sensorData); err != nil {
				return err
			}
			continue
		}

		s.logger.Error("interceptor return error: %s \t correlationId: %d", err.Error(), msg.CorrelationId)
		var response *gen.Response
		var rateLimitError *collectorDomain.RateLimitError
		if errors.As(err, &rateLimitError) {
			response = &gen.Response{
				Code:       gen.Codes_CODE_RESOURCE_EXHAUSTED,
				Message:    "rate limit exceeded",
				RetryAfter: durationpb.New(rateLimitError.Delay),
			}
		} else if errors.Is(err, collectorDomain.ErrValidation) {
			response = &gen.Response{
				Code:    gen.Codes_CODE_INVALID_ARGUMENT,
				Message: err.Error(),
			}
		} else {
			s.logger.Error("interceptors return unknown error: %s", err.Error())
			response = &gen.Response{
				Code:    gen.Codes_CODE_INTERNAL,
				Message: "Internal error",
			}
		}

		response.CorrelationId = msg.CorrelationId
		if err := stream.Send(response); err != nil {
			return fmt.Errorf("error sending response: %w", err)
		}
	}
}

// NewStreamConsumer creates a new StreamConsumer with the specified interceptors and logger.
// The consumer is initialized with a buffered data channel (capacity: 10) to handle
// burst traffic and prevent blocking during message processing.
//
// Parameters:
//   - interceptors: chain of validation and processing interceptors to apply to incoming messages
//   - logger: logger instance for error reporting and debugging
//
// Returns a configured StreamConsumer ready to process gRPC streams.
// The consumer must be stopped using Stop() to properly clean up resources.
func NewStreamConsumer(
	interceptors *collectorDomain.Interceptors[gen.SensorData],
	logger collectorDomain.Logger,
) *StreamConsumer {
	return &StreamConsumer{
		interceptors: interceptors,
		logger:       logger,
		dataConsumer: make(chan *collectorDomain.SensorData, 10),
	}
}
