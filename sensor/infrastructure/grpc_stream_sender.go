package infrastructure

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"connectrpc.com/connect"
	sensorpb "github.com/samoilenko/cossack_labs/pkg/sensorpb/v1"
	sensorDomain "github.com/samoilenko/cossack_labs/sensor/domain"
)

// IDGenerator defines the contract for generating unique identifiers.
type IDGenerator interface {
	// Generate returns a unique int64 identifier.
	Generate() int64
}

// StreamManager defines the contract for managing a bidirectional gRPC stream.
type StreamManager interface {
	EstablishNewConnection(ctx context.Context) (*connect.BidiStreamForClient[sensorpb.SensorData, sensorpb.Response], error)
	Get() (*connect.BidiStreamForClient[sensorpb.SensorData, sensorpb.Response], error)
	IsReady() bool
	Close()
}

// GrpcStreamSender manages a bidirectional gRPC stream for sending sensor data with automatic reconnection and rate limiting.
type GrpcStreamSender struct {
	streamLock          sync.RWMutex
	streamManager       StreamManager
	logger              sensorDomain.Logger
	idGenerator         IDGenerator
	backgroundJobsGroup sync.WaitGroup
	reconnectCh         chan bool
	retryAfterDelay     *RetryAfterDelay
	retryAfterDelayCh   chan time.Duration
	ready               bool
}

// Run starts the stream sender and manages the gRPC stream lifecycle until context is cancelled.
// It handles automatic reconnection and rate limiting through background goroutines.
func (s *GrpcStreamSender) Run(ctx context.Context) {
	defer func() {
		withCancel, cancel := context.WithTimeout(context.Background(), time.Second*3)
		defer cancel()
		s.Stop(withCancel)
	}()

	s.triggerReconnect()
	s.backgroundJobsGroup.Add(1)
	go func() {
		defer s.backgroundJobsGroup.Done()
		s.retryAfterDelayManager(ctx)
	}()
	s.streamConnectionManager(ctx)
}

// Stop gracefully shuts down the stream sender and waits for background jobs to complete.
func (s *GrpcStreamSender) Stop(ctx context.Context) {
	s.logger.Info("Closing stream...")
	done := make(chan struct{})
	go func() {
		s.backgroundJobsGroup.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		s.logger.Error("background jobs were not stopped in time")
	case <-done:
		s.logger.Info("background jobs stopped")
	}
}

// streamManager handles stream lifecycle including connection, reconnection with exponential backoff, and response reading.
func (s *GrpcStreamSender) streamConnectionManager(ctx context.Context) {
	delay := time.Second * 1
	maxDelay := time.Second * 10

	var ctxWithCancel context.Context
	var cancel context.CancelFunc

	for {
		select {
		case <-ctx.Done():
			if cancel != nil {
				cancel()
			}

			return
		case <-s.reconnectCh:
			if cancel != nil {
				cancel()
			}

			s.logger.Info("reconnecting stream...")

			newStream, err := s.streamManager.EstablishNewConnection(ctx)
			if err != nil {
				s.logger.Error(err.Error())
				select {
				case <-ctx.Done():
					return
				case <-time.After(delay):
					if delay > maxDelay {
						delay = maxDelay
					} else {
						delay *= 2
					}
					s.triggerReconnect()
					continue
				}
			}

			// on each new stream add stream reader
			// and close old reader on creating new stream
			ctxWithCancel, cancel = context.WithCancel(ctx)
			s.backgroundJobsGroup.Add(1)
			go func() {
				defer func() {
					s.backgroundJobsGroup.Done()
				}()
				s.readStreamResponse(ctxWithCancel, newStream)
			}()
		}
	}
}

// readStreamResponse continuously reads responses from the gRPC stream and handles rate limiting and error responses.
func (s *GrpcStreamSender) readStreamResponse(ctx context.Context, stream *connect.BidiStreamForClient[sensorpb.SensorData, sensorpb.Response]) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		resp, err := stream.Receive()
		if err != nil {
			s.logger.Error("error receiving response: %s", err.Error())
			return
		}

		switch resp.Code {
		case sensorpb.Codes_CODE_RESOURCE_EXHAUSTED:
			s.logger.Info("message dropped, retry after delay: %s, \t correlationId: %d",
				resp.RetryAfter.AsDuration(), resp.CorrelationId,
			)
			s.triggerRetryAfterDelay(resp.RetryAfter.AsDuration())
		case sensorpb.Codes_CODE_INVALID_ARGUMENT, sensorpb.Codes_CODE_INTERNAL:
			s.logger.Error("server responded with error code %s: %s, \t correlationId: %d",
				resp.Code.String(), resp.Message, resp.CorrelationId,
			)
		}
	}
}

// triggerRetryAfterDelay signals the retry delay manager to set a new delay period.
func (s *GrpcStreamSender) triggerRetryAfterDelay(value time.Duration) {
	select {
	case s.retryAfterDelayCh <- value:
	default:
	}
}

// retryAfterDelayManager manages rate limiting delays by setting and resetting retry periods.
func (s *GrpcStreamSender) retryAfterDelayManager(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case value := <-s.retryAfterDelayCh:
			s.retryAfterDelay.Set(value)
			select {
			case <-ctx.Done():
				return
			case <-time.After(value):
				s.retryAfterDelay.Reset()
			}
		}
	}
}

// Send transmits sensor data through the gRPC stream, handling rate limiting and connection readiness.
// Returns a RateLimitError if rate limited, or ErrTransportNotReady if the stream is not ready.
func (s *GrpcStreamSender) Send(_ context.Context, value int32, timestamp time.Time, sensorName sensorDomain.SensorName) error {
	delayBetweenSends := s.retryAfterDelay.Get()
	if delayBetweenSends > 0 {
		return &sensorDomain.RateLimitError{
			Message: "rate limit exceeded",
			Delay:   delayBetweenSends,
		}
	}

	if !s.streamManager.IsReady() {
		s.logger.Info("Stream not ready")
		return sensorDomain.ErrTransportNotReady
	}

	stream, err := s.streamManager.Get()
	if err != nil {
		s.logger.Error("error occurred on getting stream: %s", err.Error())
		s.triggerReconnect()
		return sensorDomain.ErrTransportNotReady
	}

	sensorData := &sensorpb.SensorData{
		SensorValue:   value,
		Timestamp:     timestamppb.New(timestamp),
		SensorName:    string(sensorName),
		CorrelationId: s.idGenerator.Generate(),
	}
	s.logger.Info("sending message: %v", sensorData)
	err = stream.Send(sensorData)
	if err != nil {
		s.logger.Error("error sending message: %s, \t correlationId: %d",
			err.Error(), sensorData.CorrelationId,
		)
		s.triggerReconnect()

		return fmt.Errorf("error sending message: %w", err)
	}

	return nil
}

// triggerReconnect signals the stream manager to initiate a reconnection.
func (s *GrpcStreamSender) triggerReconnect() {
	select {
	case s.reconnectCh <- true:
	default:
	}
}

// NewGrpcStreamSender creates a new GrpcStreamSender with the specified client and logger.
func NewGrpcStreamSender(
	streamManager StreamManager,
	logger sensorDomain.Logger,
) *GrpcStreamSender {
	sender := &GrpcStreamSender{
		logger:            logger,
		streamManager:     streamManager,
		reconnectCh:       make(chan bool, 1),
		retryAfterDelayCh: make(chan time.Duration, 1),
		retryAfterDelay:   NewRetryAfterDelay(),
		idGenerator:       &AtomicIDGenerator{},
	}
	return sender
}
