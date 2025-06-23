package domain

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// Mock implementations for testing

type MockSenderLogger struct {
	mu        sync.Mutex
	infoCalls []string
}

func (m *MockSenderLogger) Error(_ string, _ ...interface{}) {}

func (m *MockSenderLogger) Info(msg string, _ ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.infoCalls = append(m.infoCalls, msg)
}

func (m *MockSenderLogger) GetInfoCalls() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string{}, m.infoCalls...)
}

func (m *MockSenderLogger) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.infoCalls = nil
}

type MockTransport struct {
	sendCalls []SendCall
	sendError error
	mu        sync.Mutex
	sendDelay time.Duration
}

type SendCall struct {
	Timestamp  time.Time
	SensorName SensorName
	Value      int32
}

func (m *MockTransport) Send(ctx context.Context, value int32, timestamp time.Time, sensorName SensorName) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.sendDelay > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(m.sendDelay):
		}
	}

	m.sendCalls = append(m.sendCalls, SendCall{
		Value:      value,
		Timestamp:  timestamp,
		SensorName: sensorName,
	})

	return m.sendError
}

func (m *MockTransport) GetSendCalls() []SendCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]SendCall{}, m.sendCalls...)
}

func (m *MockTransport) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendCalls = nil
	m.sendError = nil
	m.sendDelay = 0
}

type MockLogger struct {
	infoCalls []string
	mu        sync.Mutex
}

func (m *MockLogger) Info(msg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.infoCalls = append(m.infoCalls, msg)
}

func (m *MockLogger) GetInfoCalls() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string{}, m.infoCalls...)
}

func (m *MockLogger) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.infoCalls = nil
}

// conditionalTransport wraps MockTransport to inject a rate-limit error on the second call.
type conditionalTransport struct {
	base  *MockTransport
	count int
}

func (c *conditionalTransport) Send(ctx context.Context, value int32, timestamp time.Time, sensorName SensorName) error {
	c.count++
	if c.count == 2 {
		return &RateLimitError{Delay: 10 * time.Millisecond}
	}
	return c.base.Send(ctx, value, timestamp, sensorName)
}

func (c *conditionalTransport) GetSendCalls() []SendCall {
	return c.base.GetSendCalls()
}

// Test cases

func TestNewSensorDataSender(t *testing.T) {
	transport := &MockTransport{}
	logger := &MockSenderLogger{}
	sensorName := SensorName("test-sensor")

	sender := NewSensorDataSender(transport, logger, sensorName)

	if sender.transport != transport {
		t.Error("transport not set correctly")
	}
	if sender.logger != logger {
		t.Error("logger not set correctly")
	}
	if sender.sensorName != sensorName {
		t.Error("sensor name not set correctly")
	}
}

func TestSensorDataSender_Send_Success(t *testing.T) {
	transport := &MockTransport{}
	logger := &MockSenderLogger{}
	sensorName := SensorName("test-sensor")
	sender := NewSensorDataSender(transport, logger, sensorName)

	ctx := context.Background()
	values := make(chan *SensorValue, 2)

	timestamp1 := time.Now()
	timestamp2 := timestamp1.Add(time.Second)

	values <- &SensorValue{Value: 100, Timestamp: timestamp1}
	values <- &SensorValue{Value: 200, Timestamp: timestamp2}
	close(values)

	sender.Send(ctx, values)

	calls := transport.GetSendCalls()
	if len(calls) != 2 {
		t.Fatalf("expected 2 send calls, got %d", len(calls))
	}

	if calls[0].Value != 100 || calls[0].SensorName != sensorName {
		t.Error("first call parameters incorrect")
	}
	if calls[1].Value != 200 || calls[1].SensorName != sensorName {
		t.Error("second call parameters incorrect")
	}

	infoCalls := logger.GetInfoCalls()
	if len(infoCalls) != 0 {
		t.Errorf("expected no info calls, got %d", len(infoCalls))
	}
}

func TestSensorDataSender_Send_RateLimitError(t *testing.T) {
	transport := &MockTransport{}
	logger := &MockSenderLogger{}
	sensorName := SensorName("test-sensor")
	sender := NewSensorDataSender(transport, logger, sensorName)

	rateLimitDelay := 50 * time.Millisecond
	transport.sendError = &RateLimitError{Delay: rateLimitDelay}

	ctx := context.Background()
	values := make(chan *SensorValue, 1)
	values <- &SensorValue{Value: 100, Timestamp: time.Now()}
	close(values)

	start := time.Now()
	sender.Send(ctx, values)
	elapsed := time.Since(start)

	// Should have waited for the rate limit delay
	if elapsed < rateLimitDelay {
		t.Errorf("expected to wait at least %v, but only waited %v", rateLimitDelay, elapsed)
	}

	infoCalls := logger.GetInfoCalls()
	if len(infoCalls) != 1 {
		t.Fatalf("expected 1 info call, got %d", len(infoCalls))
	}

	expectedMsg := "data ignored due to rate limit. Next send after %s"
	if infoCalls[0] != expectedMsg {
		t.Errorf("expected log message %q, got %q", expectedMsg, infoCalls[0])
	}
}

func TestSensorDataSender_Send_TransportNotReadyError(t *testing.T) {
	transport := &MockTransport{}
	logger := &MockSenderLogger{}
	sensorName := SensorName("test-sensor")
	sender := NewSensorDataSender(transport, logger, sensorName)

	transport.sendError = ErrTransportNotReady

	ctx := context.Background()
	values := make(chan *SensorValue, 1)
	values <- &SensorValue{Value: 100, Timestamp: time.Now()}
	close(values)

	start := time.Now()
	sender.Send(ctx, values)
	elapsed := time.Since(start)

	// Should have waited for 1 second
	expectedDelay := time.Second
	if elapsed < expectedDelay {
		t.Errorf("expected to wait at least %v, but only waited %v", expectedDelay, elapsed)
	}

	// Should not log anything for transport not ready error
	infoCalls := logger.GetInfoCalls()
	if len(infoCalls) != 0 {
		t.Errorf("expected no info calls, got %d", len(infoCalls))
	}
}

func TestSensorDataSender_Send_ContextCancellation_DuringRateLimit(t *testing.T) {
	transport := &MockTransport{}
	logger := &MockSenderLogger{}
	sensorName := SensorName("test-sensor")
	sender := NewSensorDataSender(transport, logger, sensorName)

	transport.sendError = &RateLimitError{Delay: time.Hour} // Very long delay

	ctx, cancel := context.WithCancel(context.Background())
	values := make(chan *SensorValue, 1)
	values <- &SensorValue{Value: 100, Timestamp: time.Now()}
	close(values)

	done := make(chan struct{})
	go func() {
		sender.Send(ctx, values)
		close(done)
	}()

	// Cancel context after a short delay
	time.Sleep(10 * time.Millisecond)
	cancel()

	// Should return quickly after cancellation
	select {
	case <-done:
		// Good, returned after cancellation
	case <-time.After(100 * time.Millisecond):
		t.Error("Send did not return after context cancellation")
	}
}

func TestSensorDataSender_Send_ContextCancellation_DuringTransportNotReady(t *testing.T) {
	transport := &MockTransport{}
	logger := &MockSenderLogger{}
	sensorName := SensorName("test-sensor")
	sender := NewSensorDataSender(transport, logger, sensorName)

	transport.sendError = ErrTransportNotReady

	ctx, cancel := context.WithCancel(context.Background())
	values := make(chan *SensorValue, 1)
	values <- &SensorValue{Value: 100, Timestamp: time.Now()}
	close(values)

	done := make(chan struct{})
	go func() {
		sender.Send(ctx, values)
		close(done)
	}()

	// Cancel context after a short delay
	time.Sleep(10 * time.Millisecond)
	cancel()

	// Should return quickly after cancellation
	select {
	case <-done:
		// Good, returned after cancellation
	case <-time.After(100 * time.Millisecond):
		t.Error("Send did not return after context cancellation")
	}
}

func TestSensorDataSender_Send_EmptyChannel(t *testing.T) {
	transport := &MockTransport{}
	logger := &MockSenderLogger{}
	sensorName := SensorName("test-sensor")
	sender := NewSensorDataSender(transport, logger, sensorName)

	ctx := context.Background()
	values := make(chan *SensorValue)
	close(values) // Empty channel

	sender.Send(ctx, values)

	calls := transport.GetSendCalls()
	if len(calls) != 0 {
		t.Errorf("expected no send calls, got %d", len(calls))
	}
}

func TestSensorDataSender_Send_OtherError(t *testing.T) {
	transport := &MockTransport{}
	logger := &MockSenderLogger{}
	sensorName := SensorName("test-sensor")
	sender := NewSensorDataSender(transport, logger, sensorName)

	otherError := errors.New("some other error")
	transport.sendError = otherError

	ctx := context.Background()
	values := make(chan *SensorValue, 1)
	values <- &SensorValue{Value: 100, Timestamp: time.Now()}
	close(values)

	sender.Send(ctx, values)

	// For other errors, the function should continue without special handling
	calls := transport.GetSendCalls()
	if len(calls) != 1 {
		t.Errorf("expected 1 send call, got %d", len(calls))
	}

	infoCalls := logger.GetInfoCalls()
	if len(infoCalls) != 0 {
		t.Errorf("expected no info calls for other errors, got %d", len(infoCalls))
	}
}

func TestSensorDataSender_Send_MixedScenarios(t *testing.T) {
	base := &MockTransport{}
	transport := &conditionalTransport{base: base}
	logger := &MockSenderLogger{}
	sensorName := SensorName("test-sensor")
	sender := NewSensorDataSender(transport, logger, sensorName)

	ctx := context.Background()
	values := make(chan *SensorValue, 3)

	values <- &SensorValue{Value: 100, Timestamp: time.Now()}
	values <- &SensorValue{Value: 200, Timestamp: time.Now()}
	values <- &SensorValue{Value: 300, Timestamp: time.Now()}
	close(values)

	sender.Send(ctx, values)

	calls := transport.GetSendCalls()
	if len(calls) != 2 {
		t.Errorf("expected 2 send calls, got %d", len(calls))
	}

	infoCalls := logger.GetInfoCalls()
	if len(infoCalls) != 1 {
		t.Errorf("expected 1 info call, got %d", len(infoCalls))
	}
}
