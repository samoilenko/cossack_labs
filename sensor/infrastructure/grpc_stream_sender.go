package infrastructure

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"connectrpc.com/connect"
	sensorpb "github.com/samoilenko/cossack_labs/pkg/sensorpb/v1"
	sensorConnect "github.com/samoilenko/cossack_labs/pkg/sensorpb/v1/sensorpbv1connect"
	sensorDomain "github.com/samoilenko/cossack_labs/sensor/domain"
)

// IDGenerator defines the contract for generating unique identifiers.
type IDGenerator interface {
	// Generate returns a unique int64 identifier.
	Generate() int64
}

// GrpcStreamSender manages a bidirectional gRPC stream for sending sensor data with automatic reconnection and rate limiting.
type GrpcStreamSender struct {
	streamLock          sync.RWMutex
	client              sensorConnect.SensorServiceClient
	logger              sensorDomain.Logger
	idGenerator         IDGenerator
	backgroundJobsGroup sync.WaitGroup
	stream              *connect.BidiStreamForClient[sensorpb.SensorData, sensorpb.Response]
	readyLock           sync.Mutex
	reconnectCh         chan bool
	retryAfterDelay     *RetryAfterDelay
	retryAfterDelayCh   chan time.Duration
	ready               bool
}

// Run starts the stream sender and manages the gRPC stream lifecycle until context is cancelled.
// It handles automatic reconnection and rate limiting through background goroutines.
func (s *GrpcStreamSender) Run(ctx context.Context) {
	defer func() {
		withCancel, cancel := context.WithTimeout(context.Background(), time.Second*2)
		defer cancel()
		s.Stop(withCancel)
	}()

	s.triggerReconnect()
	s.backgroundJobsGroup.Add(1)
	go func() {
		defer s.backgroundJobsGroup.Done()
		s.retryAfterDelayManager(ctx)
	}()

	s.streamManager(ctx)
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
		s.logger.Error("background jobs was not stopped in time")
	case <-done:
		s.logger.Info("background jobs stopped")
	}
}

// streamManager handles stream lifecycle including connection, reconnection with exponential backoff, and response reading.
func (s *GrpcStreamSender) streamManager(ctx context.Context) {
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
			s.setReady(false)
			s.closeStream()

			newStream := s.client.GetStream(ctx)
			_, err := newStream.Conn()
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
			s.setStream(newStream)
			ctxWithCancel, cancel = context.WithCancel(ctx)
			s.backgroundJobsGroup.Add(1)
			go func() {
				s.backgroundJobsGroup.Done()
				s.readStreamResponse(ctxWithCancel, newStream)
			}()
			s.setReady(true)
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

// setReady updates the ready state of the stream sender.
func (s *GrpcStreamSender) setReady(value bool) {
	s.readyLock.Lock()
	defer s.readyLock.Unlock()
	s.ready = value
}

// isReady returns the current ready state of the stream sender.
func (s *GrpcStreamSender) isReady() bool {
	s.readyLock.Lock()
	defer s.readyLock.Unlock()
	return s.ready
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

	if !s.isReady() {
		s.logger.Info("Stream not ready")
		return sensorDomain.ErrTransportNotReady
	}

	stream := s.getStream()
	if stream == nil {
		s.logger.Error("stream is nil, triggering reconnect")
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
	err := stream.Send(sensorData)
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

// getStream return instance of GRPC stream
func (s *GrpcStreamSender) getStream() *connect.BidiStreamForClient[sensorpb.SensorData, sensorpb.Response] {
	s.streamLock.RLock()
	defer s.streamLock.RUnlock()
	return s.stream
}

// setStream updates the current stream.
func (s *GrpcStreamSender) setStream(stream *connect.BidiStreamForClient[sensorpb.SensorData, sensorpb.Response]) {
	s.streamLock.Lock()
	defer s.streamLock.Unlock()
	s.stream = stream
}

// closeStream closes the current stream and sets it to nil.
func (s *GrpcStreamSender) closeStream() {
	s.streamLock.Lock()
	defer s.streamLock.Unlock()
	if s.stream != nil {
		_ = s.stream.CloseRequest()
		s.stream = nil
	}
}

// NewGrpcStreamSender creates a new GrpcStreamSender with the specified client and logger.
func NewGrpcStreamSender(client sensorConnect.SensorServiceClient, logger sensorDomain.Logger) *GrpcStreamSender {
	sender := &GrpcStreamSender{
		logger:            logger,
		client:            client,
		reconnectCh:       make(chan bool, 1),
		retryAfterDelayCh: make(chan time.Duration, 1),
		retryAfterDelay:   NewRetryAfterDelay(),
		idGenerator:       &AtomicIDGenerator{},
	}
	return sender
}
