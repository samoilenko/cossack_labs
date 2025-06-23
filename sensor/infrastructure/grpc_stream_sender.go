package infrastructure

import (
	"context"
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
	Send(ctx context.Context, data *sensorpb.SensorData) error
}

// GrpcStreamSender manages a bidirectional gRPC stream for sending sensor data with automatic reconnection and rate limiting.
type GrpcStreamSender struct {
	streamLock          sync.RWMutex
	streamManager       StreamManager
	logger              sensorDomain.Logger
	idGenerator         IDGenerator
	backgroundJobsGroup sync.WaitGroup
	responseCh          chan *sensorpb.Response
	reconnectCh         chan bool
	retryAfter          *RetryAfterDelay
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
	s.streamConnectionManager(ctx)
}

// Stop gracefully shuts down the stream sender and waits for background jobs to complete.
func (s *GrpcStreamSender) Stop(ctx context.Context) {
	s.logger.Info("Closing stream sender...")
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
	close(s.responseCh)
}

// streamManager handles stream lifecycle including connection, reconnection with exponential backoff, and response reading.
func (s *GrpcStreamSender) streamConnectionManager(ctx context.Context) {
	delay := time.Second * 1
	maxDelay := time.Second * 10

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
				s.logger.Error("error on establishing new stream: %s", err.Error())
				select {
				case <-ctx.Done():
					if cancel != nil {
						cancel()
					}

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

			delay = time.Second * 1

			// on each new stream add stream reader
			// and close old reader on creating new stream
			streamCtx, newCancel := context.WithCancel(ctx)
			cancel = newCancel
			s.backgroundJobsGroup.Add(1)
			go func() {
				defer s.backgroundJobsGroup.Done()
				s.readStreamResponse(streamCtx, newStream)
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
		s.responseCh <- resp
	}
}

// Send transmits sensor data through the gRPC stream, handling rate limiting and connection readiness.
// Returns a RateLimitError if rate limited, or ErrTransportNotReady if the stream is not ready.
func (s *GrpcStreamSender) Send(ctx context.Context, value int32, timestamp time.Time, sensorName sensorDomain.SensorName) error {
	delayBetweenSends := s.retryAfter.Get()
	if delayBetweenSends > 0 {
		return &sensorDomain.RateLimitError{
			Message: "rate limit exceeded",
			Delay:   delayBetweenSends,
		}
	}

	sensorData := &sensorpb.SensorData{
		SensorValue:   value,
		Timestamp:     timestamppb.New(timestamp),
		SensorName:    string(sensorName),
		CorrelationId: s.idGenerator.Generate(),
	}
	s.logger.Info("sending message: %v", sensorData)
	err := s.streamManager.Send(ctx, sensorData)
	if err != nil {
		s.logger.Error("error sending message: %s, \t correlationId: %d",
			err.Error(), sensorData.CorrelationId,
		)
		s.triggerReconnect()

		return sensorDomain.ErrTransportNotReady
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

func (s *GrpcStreamSender) GetResponseChannel() <-chan *sensorpb.Response {
	return s.responseCh
}

// NewGrpcStreamSender creates a new GrpcStreamSender with the specified client and logger.
func NewGrpcStreamSender(
	streamManager StreamManager,
	logger sensorDomain.Logger,
	retryAfter *RetryAfterDelay,
) *GrpcStreamSender {
	sender := &GrpcStreamSender{
		logger:        logger,
		streamManager: streamManager,
		reconnectCh:   make(chan bool, 1),
		retryAfter:    retryAfter,
		idGenerator:   &AtomicIDGenerator{},
		responseCh:    make(chan *sensorpb.Response, 10),
	}
	return sender
}
