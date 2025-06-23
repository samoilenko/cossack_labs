package infrastructure

import (
	"context"
	"sync"
	"time"

	"connectrpc.com/connect"
	sensorpb "github.com/samoilenko/cossack_labs/pkg/sensorpb/v1"
	sensorConnect "github.com/samoilenko/cossack_labs/pkg/sensorpb/v1/sensorpbv1connect"
	sensorDomain "github.com/samoilenko/cossack_labs/sensor/domain"
)

// GrpcStream manages a bidirectional gRPC stream.
type GrpcStream struct {
	streamLock sync.RWMutex
	readyLock  sync.RWMutex
	client     sensorConnect.SensorServiceClient
	logger     sensorDomain.Logger
	stream     *connect.BidiStreamForClient[sensorpb.SensorData, sensorpb.Response]

	ready bool
}

func (s *GrpcStream) handleBackoff(ctx context.Context, delay *time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(*delay):
		return true
	}
}

// EstablishNewConnection handles stream connection.
func (s *GrpcStream) EstablishNewConnection(ctx context.Context) (*connect.BidiStreamForClient[sensorpb.SensorData, sensorpb.Response], error) {
	delay := time.Second * 1
	maxDelay := time.Second * 10

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		s.logger.Info("getting new stream...")
		s.setReady(false)

		if delay > maxDelay {
			delay = maxDelay
		} else {
			delay *= 2
		}

		newStream := s.client.GetStream(ctx)
		_, err := newStream.Conn()
		if err != nil {
			s.logger.Error("got an error on getting stream: %s", err.Error())
			if s.handleBackoff(ctx, &delay) {
				continue
			}
			return nil, err
		}

		s.setStream(newStream)
		s.setReady(true)

		return newStream, nil
	}
}

// Get returns instance of GRPC stream
func (s *GrpcStream) get() (*connect.BidiStreamForClient[sensorpb.SensorData, sensorpb.Response], error) {
	if !s.IsReady() {
		return nil, sensorDomain.ErrTransportNotReady
	}

	s.streamLock.RLock()
	defer s.streamLock.RUnlock()
	return s.stream, nil
}

// IsReady returns the current ready state of the stream sender.
func (s *GrpcStream) IsReady() bool {
	s.readyLock.RLock()
	defer s.readyLock.RUnlock()
	return s.ready
}

// Send sends data through the stream.
func (s *GrpcStream) Send(ctx context.Context, data *sensorpb.SensorData) error {
	stream, err := s.get()
	if err != nil {
		return err
	}

	done := make(chan error, 1)
	go func() {
		done <- stream.Send(data)
	}()

	select {
	case <-ctx.Done():
		stream.CloseRequest()
		return ctx.Err()
	case sendErr := <-done:
		return sendErr
	}
}

// setReady updates the ready state of the stream sender.
func (s *GrpcStream) setReady(value bool) {
	s.readyLock.Lock()
	defer s.readyLock.Unlock()
	s.ready = value
}

// setStream updates the current stream.
func (s *GrpcStream) setStream(stream *connect.BidiStreamForClient[sensorpb.SensorData, sensorpb.Response]) {
	s.streamLock.Lock()
	defer s.streamLock.Unlock()
	s.stream = stream
}

// NewGrpcStream creates a new GrpcStreamSender with the specified client and logger.
func NewGrpcStream(client sensorConnect.SensorServiceClient, logger sensorDomain.Logger) *GrpcStream {
	return &GrpcStream{
		logger: logger,
		client: client,
	}
}
