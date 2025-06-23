package infrastructure

import (
	"context"
	"testing"
	"time"

	collectorDomain "github.com/samoilenko/cossack_labs/collector/domain"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func createTestMessage(dataSize int) *anypb.Any {
	// Create some test data
	data := make([]byte, dataSize)
	for i := range data {
		data[i] = byte('a')
	}

	// Create an Any message with the data
	message := &anypb.Any{
		TypeUrl: "type.googleapis.com/test.Message",
		Value:   data,
	}
	return message
}

func TestNewRateLimiter(t *testing.T) {
	maxLimit := collectorDomain.RateLimit(1000)
	rateLimiter := NewRateLimiter[*anypb.Any](maxLimit, 1*time.Second)

	if rateLimiter == nil {
		t.Fatal("NewRateLimiter returned nil")
	}

	if rateLimiter.maxLimit != maxLimit {
		t.Errorf("Expected maxLimit %d, got %d", maxLimit, rateLimiter.maxLimit)
	}

	if rateLimiter.limit != 0 {
		t.Errorf("Expected initial limit 0, got %d", rateLimiter.limit)
	}

	if !rateLimiter.last.IsZero() {
		t.Errorf("Expected initial last time to be zero, got %v", rateLimiter.last)
	}
}

func TestRateLimiter_Apply_FirstMessage(t *testing.T) {
	maxLimit := collectorDomain.RateLimit(1000)
	rateLimiter := NewRateLimiter[*anypb.Any](maxLimit, 1*time.Second)

	msg := createTestMessage(100)
	err := rateLimiter.Apply(msg)

	if err != nil {
		t.Errorf("Expected no error for first message, got %v", err)
	}

	expectedSize := proto.Size(msg)
	if rateLimiter.limit != expectedSize {
		t.Errorf("Expected limit to be message size %d, got %d", expectedSize, rateLimiter.limit)
	}

	if rateLimiter.last.IsZero() {
		t.Error("Expected last time to be set after first message")
	}
}

func TestRateLimiter_Apply_WithinTimeWindow_UnderLimit(t *testing.T) {
	maxLimit := collectorDomain.RateLimit(1000)
	rateLimiter := NewRateLimiter[*anypb.Any](maxLimit, 1*time.Second)

	// First message
	msg1 := createTestMessage(300)
	err := rateLimiter.Apply(msg1)
	if err != nil {
		t.Fatalf("Unexpected error on first message: %v", err)
	}

	// Second message within the same second, total under limit
	msg2 := createTestMessage(400)
	err = rateLimiter.Apply(msg2)
	if err != nil {
		t.Errorf("Expected no error when under limit, got %v", err)
	}

	expectedLimit := proto.Size(msg1) + proto.Size(msg2)
	if rateLimiter.limit != expectedLimit {
		t.Errorf("Expected limit %d, got %d", expectedLimit, rateLimiter.limit)
	}
}

func TestRateLimiter_Apply_WithinTimeWindow_OverLimit(t *testing.T) {
	maxLimit := collectorDomain.RateLimit(500)
	rateLimiter := NewRateLimiter[*anypb.Any](maxLimit, 1*time.Second)

	// First message
	msg1 := createTestMessage(300)
	err := rateLimiter.Apply(msg1)
	if err != nil {
		t.Fatalf("Unexpected error on first message: %v", err)
	}

	// Second message that would exceed the limit
	msg2 := createTestMessage(300)
	err = rateLimiter.Apply(msg2)

	if err == nil {
		t.Error("Expected rate limit error when over limit")
	}

	// Check if it's the correct error type
	rateLimitErr, ok := err.(*collectorDomain.RateLimitError)
	if !ok {
		t.Errorf("Expected RateLimitError, got %T", err)
	} else if rateLimitErr.Delay < 0 {
		t.Errorf("Expected positive delay, got %v", rateLimitErr.Delay)
	}
}

func TestRateLimiter_Apply_AfterTimeWindow_Reset(t *testing.T) {
	maxLimit := collectorDomain.RateLimit(500)
	rateLimiter := NewRateLimiter[*anypb.Any](maxLimit, 1*time.Second)

	// First message
	msg1 := createTestMessage(400)
	err := rateLimiter.Apply(msg1)
	if err != nil {
		t.Fatalf("Unexpected error on first message: %v", err)
	}

	// Simulate time passing (more than 1 second)
	rateLimiter.last = time.Now().Add(-2 * time.Second)

	// Second message after time window - should reset
	msg2 := createTestMessage(400)
	err = rateLimiter.Apply(msg2)
	if err != nil {
		t.Errorf("Expected no error after time window reset, got %v", err)
	}

	// The limit should be reset to just the size of msg2
	expectedSize := proto.Size(msg2)
	if rateLimiter.limit != expectedSize {
		t.Errorf("Expected limit to be reset to %d, got %d", expectedSize, rateLimiter.limit)
	}
}

func TestRateLimiter_Apply_ExactlyAtLimit(t *testing.T) {
	maxLimit := collectorDomain.RateLimit(674)
	rateLimiter := NewRateLimiter[*anypb.Any](maxLimit, 1*time.Second)

	// First message
	msg1 := createTestMessage(300)
	err := rateLimiter.Apply(msg1)
	if err != nil {
		t.Fatalf("Unexpected error on first message: %v", err)
	}

	// Second message that brings us exactly to the limit
	msg2 := createTestMessage(300)
	err = rateLimiter.Apply(msg2)
	if err != nil {
		t.Errorf("Expected no error when exactly at limit, got %v", err)
	}

	// Third message that exceeds the limit
	msg3 := createTestMessage(1)
	err = rateLimiter.Apply(msg3)
	if err == nil {
		t.Error("Expected rate limit error when exceeding limit")
	}
}

func TestRateLimiter_Apply_ConcurrentAccess(t *testing.T) {
	maxLimit := collectorDomain.RateLimit(1000)
	rateLimiter := NewRateLimiter[*anypb.Any](maxLimit, 1*time.Second)

	// Create context with timeout to detect deadlocks
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const numGoroutines = 10
	done := make(chan bool, numGoroutines)
	completed := make(chan struct{})

	// Run concurrent operations
	go func() {
		defer close(completed)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				msg := createTestMessage(50)
				result := rateLimiter.Apply(msg)
				_ = result // Use the result to prevent optimization
				done <- true
			}()
		}

		// Wait for all goroutines to complete
		for i := 0; i < numGoroutines; i++ {
			<-done
		}
	}()

	// Wait for completion or timeout
	select {
	case <-completed:
		t.Log("Concurrent access test completed successfully without deadlock")
	case <-ctx.Done():
		t.Fatal("Test timed out - possible deadlock in concurrent Apply operations")
	}
}

func TestRateLimiter_Apply_ZeroSizeMessage(t *testing.T) {
	maxLimit := collectorDomain.RateLimit(1000)
	rateLimiter := NewRateLimiter[*anypb.Any](maxLimit, 1*time.Second)

	msg := createTestMessage(0)
	err := rateLimiter.Apply(msg)

	if err != nil {
		t.Errorf("Expected no error for zero-size message, got %v", err)
	}

	// Note: proto.Size() might return a small value even for "empty" messages
	// due to protobuf encoding overhead, so we just check it's reasonable
	if rateLimiter.limit < 0 || rateLimiter.limit > 100 {
		t.Errorf("Expected limit to be small for zero-size message, got %d", rateLimiter.limit)
	}
}

func TestRateLimiter_Apply_EdgeCaseTimeWindow(t *testing.T) {
	maxLimit := collectorDomain.RateLimit(1000)
	rateLimiter := NewRateLimiter[*anypb.Any](maxLimit, 1*time.Second)

	// First message
	msg1 := createTestMessage(500)
	err := rateLimiter.Apply(msg1)
	if err != nil {
		t.Fatalf("Unexpected error on first message: %v", err)
	}

	// Set time to exactly 1 second later
	rateLimiter.last = time.Now().Add(-1 * time.Second)

	// Second message exactly at the 1-second boundary
	msg2 := createTestMessage(500)
	err = rateLimiter.Apply(msg2)
	if err != nil {
		t.Errorf("Expected no error at 1-second boundary, got %v", err)
	}

	// The limit should be reset since elapsed >= 1 second
	expectedSize := proto.Size(msg2)
	if rateLimiter.limit != expectedSize {
		t.Errorf("Expected limit to be reset to %d, got %d", expectedSize, rateLimiter.limit)
	}
}

func TestRateLimiter_Apply_RealWorldScenario(t *testing.T) {
	maxLimit := collectorDomain.RateLimit(1000)
	rateLimiter := NewRateLimiter[*anypb.Any](maxLimit, 1*time.Second)

	// Simulate a burst of messages
	messages := []*anypb.Any{
		createTestMessage(200),
		createTestMessage(300),
		createTestMessage(400), // This should trigger rate limiting
	}

	var successCount, rateLimitedCount int

	for _, msg := range messages {
		err := rateLimiter.Apply(msg)
		if err != nil {
			if _, ok := err.(*collectorDomain.RateLimitError); ok {
				rateLimitedCount++
			} else {
				t.Errorf("Unexpected error type: %T", err)
			}
		} else {
			successCount++
		}
	}

	if successCount != 2 {
		t.Errorf("Expected 2 successful messages, got %d", successCount)
	}

	if rateLimitedCount != 1 {
		t.Errorf("Expected 1 rate-limited message, got %d", rateLimitedCount)
	}
}

// Benchmark tests
func BenchmarkRateLimiter_Apply(b *testing.B) {
	maxLimit := collectorDomain.RateLimit(10000)
	rateLimiter := NewRateLimiter[*anypb.Any](maxLimit, 1*time.Second)
	msg := createTestMessage(100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = rateLimiter.Apply(msg)
	}
}

func BenchmarkRateLimiter_Apply_Concurrent(b *testing.B) {
	maxLimit := collectorDomain.RateLimit(10000)
	rateLimiter := NewRateLimiter[*anypb.Any](maxLimit, 1*time.Second)
	msg := createTestMessage(100)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = rateLimiter.Apply(msg)
		}
	})
}

func BenchmarkRateLimiter_MessageCreation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		createTestMessage(100)
	}
}
