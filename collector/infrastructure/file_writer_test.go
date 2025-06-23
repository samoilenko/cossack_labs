package infrastructure

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	collectorDomain "github.com/samoilenko/cossack_labs/collector/domain"
)

// Mock logger for testing
type mockLogger struct {
	mu       sync.Mutex
	messages []string
}

func (m *mockLogger) Error(msg string, _ ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, msg)
}

func (m *mockLogger) Info(_ string, _ ...interface{}) {}

func (m *mockLogger) GetMessages() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string{}, m.messages...)
}

func (m *mockLogger) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = nil
}

// Test helper to create temporary file
func createTempFile(tb testing.TB) (string, func()) {
	tb.Helper()
	tmpDir, err := os.MkdirTemp("", "filewriter_test")
	if err != nil {
		tb.Fatalf("Failed to create temp dir: %v", err)
	}

	filePath := filepath.Join(tmpDir, "test.log")
	cleanup := func() {
		_ = os.RemoveAll(tmpDir)
	}

	return filePath, cleanup
}

func TestNewFileWriter(t *testing.T) {
	logger := &mockLogger{}
	fw := NewFileWriter(
		collectorDomain.LogPath("/tmp/test.log"),
		collectorDomain.BufferSize(1024),
		collectorDomain.FlushInterval(time.Second),
		logger,
	)

	if fw == nil {
		t.Fatal("NewFileWriter returned nil")
	}

	if fw.IsReady() {
		t.Error("FileWriter should not be ready initially")
	}

	if fw.logPath != "/tmp/test.log" {
		t.Errorf("Expected logPath '/tmp/test.log', got '%s'", fw.logPath)
	}

	if fw.bufferSize != 1024 {
		t.Errorf("Expected bufferSize 1024, got %d", fw.bufferSize)
	}

	if time.Duration(fw.flushInterval) != time.Second {
		t.Errorf("Expected flushInterval 1s, got %v", fw.flushInterval)
	}
}

func TestFileWriter_IsReady_SetReady(t *testing.T) {
	logger := &mockLogger{}
	fw := NewFileWriter("", 0, 0, logger)

	// Test initial state
	if fw.IsReady() {
		t.Error("FileWriter should not be ready initially")
	}

	// Test setting ready to true
	fw.setReady(true)
	if !fw.IsReady() {
		t.Error("FileWriter should be ready after setReady(true)")
	}

	// Test setting ready to false
	fw.setReady(false)
	if fw.IsReady() {
		t.Error("FileWriter should not be ready after setReady(false)")
	}
}

func TestFileWriter_IsReady_Concurrent(t *testing.T) {
	logger := &mockLogger{}
	fw := NewFileWriter("", 0, 0, logger)

	// Create a context with timeout to detect deadlocks
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	const numGoroutines = 10

	// Channel to signal completion
	done := make(chan struct{})

	// Start the concurrent operations
	go func() {
		defer close(done)

		// Test concurrent access to IsReady/setReady
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(val bool) {
				defer wg.Done()
				fw.setReady(val)
				_ = fw.IsReady()
			}(i%2 == 0)
		}

		wg.Wait()
	}()

	// Wait for either completion or timeout
	select {
	case <-done:
		// Test completed successfully - no deadlock
		t.Log("Concurrent access test completed without deadlock")
	case <-ctx.Done():
		// Test timed out - likely deadlock
		t.Fatal("Test timed out - possible deadlock in concurrent IsReady/setReady operations")
	}
}

func TestFileWriter_OpenBufferedFile(t *testing.T) {
	logger := &mockLogger{}
	fw := NewFileWriter("", 1024, 0, logger)

	filePath, cleanup := createTempFile(t)
	defer cleanup()

	f, w, err := fw.openBufferedFile(filePath, 1024)
	if err != nil {
		t.Fatalf("openBufferedFile failed: %v", err)
	}
	defer func() {
		_ = f.Close()
	}()

	if f == nil {
		t.Error("File should not be nil")
	}
	if w == nil {
		t.Error("Writer should not be nil")
	}

	// Test writing to verify it works
	testData := "test data"
	_, err = w.WriteString(testData)
	if err != nil {
		t.Errorf("Failed to write test data: %v", err)
	}
}

func TestFileWriter_OpenBufferedFile_InvalidPath(t *testing.T) {
	logger := &mockLogger{}
	fw := NewFileWriter("", 1024, 0, logger)

	// Try to open file in non-existent directory
	_, _, err := fw.openBufferedFile("/nonexistent/path/file.log", 1024)
	if err == nil {
		t.Error("Expected error for invalid path, got nil")
	}
}

func TestFileWriter_Connect(t *testing.T) {
	logger := &mockLogger{}
	filePath, cleanup := createTempFile(t)
	defer cleanup()

	fw := NewFileWriter(
		collectorDomain.LogPath(filePath),
		collectorDomain.BufferSize(1024),
		0,
		logger,
	)

	ctx := context.Background()
	err := fw.connect(ctx)
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}

	if fw.f == nil {
		t.Error("File should be set after connect")
	}
	if fw.w == nil {
		t.Error("Writer should be set after connect")
	}

	_ = fw.Close()
}

func TestFileWriter_Connect_InvalidPath(t *testing.T) {
	logger := &mockLogger{}
	fw := NewFileWriter(
		collectorDomain.LogPath("/nonexistent/path/file.log"),
		collectorDomain.BufferSize(1024),
		0,
		logger,
	)

	ctx := context.Background()
	err := fw.connect(ctx)
	if err == nil {
		t.Error("Expected error for invalid path, got nil")
	}

	messages := logger.GetMessages()
	if len(messages) == 0 {
		t.Error("Expected error to be logged")
	}
}

func TestFileWriter_Write_NotReady(t *testing.T) {
	logger := &mockLogger{}
	fw := NewFileWriter("", 0, 0, logger)

	err := fw.Write([]byte("test"))
	if err == nil {
		t.Error("Expected error when writing to not ready FileWriter")
	}

	expectedMsg := "file writer is not ready"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error message to contain '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestFileWriter_Write_Success(t *testing.T) {
	logger := &mockLogger{}
	filePath, cleanup := createTempFile(t)
	defer cleanup()

	fw := NewFileWriter(
		collectorDomain.LogPath(filePath),
		collectorDomain.BufferSize(1024),
		0,
		logger,
	)

	ctx := context.Background()
	err := fw.connect(ctx)
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer func() {
		_ = fw.Close()
	}()

	fw.setReady(true)

	testData := []byte("Hello, World!")
	err = fw.Write(testData)
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}

	// Flush to ensure data is written
	err = fw.flush()
	if err != nil {
		t.Errorf("Flush failed: %v", err)
	}

	// Verify data was written
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(content) != string(testData) {
		t.Errorf("Expected file content '%s', got '%s'", string(testData), string(content))
	}
}

func TestFileWriter_Write_Concurrent(t *testing.T) {
	logger := &mockLogger{}
	filePath, cleanup := createTempFile(t)
	defer cleanup()

	fw := NewFileWriter(
		collectorDomain.LogPath(filePath),
		collectorDomain.BufferSize(1024),
		0,
		logger,
	)

	ctx := context.Background()
	err := fw.connect(ctx)
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer func() {
		_ = fw.Close()
	}()

	fw.setReady(true)

	var wg sync.WaitGroup
	const numGoroutines = 10
	const messagesPerGoroutine = 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < messagesPerGoroutine; j++ {
				data := fmt.Sprintf("goroutine-%d-message-%d\n", id, j)
				err := fw.Write([]byte(data))
				if err != nil {
					t.Errorf("Write failed in goroutine %d: %v", id, err)
				}
			}
		}(i)
	}

	wg.Wait()

	// Flush and verify
	err = fw.flush()
	if err != nil {
		t.Errorf("Flush failed: %v", err)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	lines := strings.Split(string(content), "\n")
	// -1 because last line is empty
	expectedLines := numGoroutines * messagesPerGoroutine
	actualLines := len(lines) - 1

	if actualLines != expectedLines {
		t.Errorf("Expected %d lines, got %d", expectedLines, actualLines)
	}
}

func TestFileWriter_Flush(t *testing.T) {
	logger := &mockLogger{}
	filePath, cleanup := createTempFile(t)
	defer cleanup()

	fw := NewFileWriter(
		collectorDomain.LogPath(filePath),
		collectorDomain.BufferSize(1024),
		0,
		logger,
	)

	ctx := context.Background()
	err := fw.connect(ctx)
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer func() {
		_ = fw.Close()
	}()

	fw.setReady(true)

	// Write data
	testData := []byte("test data")
	err = fw.Write(testData)
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}

	// Data should not be in file yet (buffered)
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if len(content) > 0 {
		t.Error("Data should not be written to file before flush")
	}

	// Flush should write data
	err = fw.flush()
	if err != nil {
		t.Errorf("Flush failed: %v", err)
	}

	// Now data should be in file
	content, err = os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if string(content) != string(testData) {
		t.Errorf("Expected file content '%s', got '%s'", string(testData), string(content))
	}
}

func TestFileWriter_Close(t *testing.T) {
	logger := &mockLogger{}
	filePath, cleanup := createTempFile(t)
	defer cleanup()

	fw := NewFileWriter(
		collectorDomain.LogPath(filePath),
		collectorDomain.BufferSize(1024),
		0,
		logger,
	)

	ctx := context.Background()
	err := fw.connect(ctx)
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}

	fw.setReady(true)

	// Write some data
	testData := []byte("test data for close")
	err = fw.Write(testData)
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}

	// Close should flush and close
	err = fw.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Verify data was flushed
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if string(content) != string(testData) {
		t.Errorf("Expected file content '%s', got '%s'", string(testData), string(content))
	}

	// Multiple closes should not panic
	err = fw.Close()
	if err != nil {
		t.Errorf("Second Close failed: %v", err)
	}
}

func TestFileWriter_Reconnect(t *testing.T) {
	logger := &mockLogger{}
	filePath, cleanup := createTempFile(t)
	defer cleanup()

	fw := NewFileWriter(
		collectorDomain.LogPath(filePath),
		collectorDomain.BufferSize(1024),
		0,
		logger,
	)

	ctx := context.Background()

	// Initial connect
	err := fw.connect(ctx)
	if err != nil {
		t.Fatalf("Initial connect failed: %v", err)
	}
	fw.setReady(true)

	// Write some data
	testData1 := []byte("before reconnect")
	err = fw.Write(testData1)
	if err != nil {
		t.Errorf("Write before reconnect failed: %v", err)
	}

	// Reconnect
	err = fw.Reconnect(ctx)
	if err != nil {
		t.Errorf("Reconnect failed: %v", err)
	}

	// Should be ready after reconnect
	if !fw.IsReady() {
		t.Error("FileWriter should be ready after reconnect")
	}

	// Write more data
	testData2 := []byte("after reconnect")
	err = fw.Write(testData2)
	if err != nil {
		t.Errorf("Write after reconnect failed: %v", err)
	}

	_ = fw.Close()

	// Verify both data pieces are in file
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	expectedContent := string(testData1) + string(testData2)
	if string(content) != expectedContent {
		t.Errorf("Expected file content '%s', got '%s'", expectedContent, string(content))
	}
}

func TestFileWriter_Start_WithContext(t *testing.T) {
	logger := &mockLogger{}
	filePath, cleanup := createTempFile(t)
	defer cleanup()

	fw := NewFileWriter(
		collectorDomain.LogPath(filePath),
		collectorDomain.BufferSize(1024),
		collectorDomain.FlushInterval(50*time.Millisecond), // Fast flush for testing
		logger,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Start in goroutine
	var startErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		fw.Start(ctx)
	}()

	// Wait a bit for Start to initialize
	time.Sleep(10 * time.Millisecond)

	// Should be ready
	if !fw.IsReady() {
		t.Error("FileWriter should be ready after Start")
	}

	// Write some data
	testData := []byte("test data during start")
	err := fw.Write(testData)
	if err != nil {
		t.Errorf("Write during Start failed: %v", err)
	}

	// Wait for context timeout
	<-done

	if startErr != nil {
		t.Errorf("Start returned error: %v", startErr)
	}

	// Verify data was written and flushed
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if string(content) != string(testData) {
		t.Errorf("Expected file content '%s', got '%s'", string(testData), string(content))
	}
}

func TestFileWriter_Start_ConnectError(t *testing.T) {
	logger := &mockLogger{}
	fw := NewFileWriter(
		collectorDomain.LogPath("/nonexistent/path/file.log"),
		collectorDomain.BufferSize(1024),
		collectorDomain.FlushInterval(time.Hour), // Long interval to avoid flush errors
		logger,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		fw.Start(ctx)
	}()

	<-done

	// Should log connection error
	messages := logger.GetMessages()
	if len(messages) == 0 {
		t.Error("Expected connection error to be logged")
	}

	foundConnectError := false
	for _, msg := range messages {
		if strings.Contains(msg, "error on connecting") {
			foundConnectError = true
			break
		}
	}
	if !foundConnectError {
		t.Error("Expected 'error on connecting' message in logs")
	}
}

// Benchmark tests
func BenchmarkFileWriter_Write(b *testing.B) {
	logger := &mockLogger{}
	filePath, cleanup := createTempFile(b)
	defer cleanup()

	fw := NewFileWriter(
		collectorDomain.LogPath(filePath),
		collectorDomain.BufferSize(8192),
		0,
		logger,
	)

	ctx := context.Background()
	err := fw.connect(ctx)
	if err != nil {
		b.Fatalf("connect failed: %v", err)
	}
	defer func() {
		_ = fw.Close()
	}()

	fw.setReady(true)
	testData := []byte("benchmark test data line\n")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := fw.Write(testData)
		if err != nil {
			b.Errorf("Write failed: %v", err)
		}
	}
}

func BenchmarkFileWriter_Write_Parallel(b *testing.B) {
	logger := &mockLogger{}
	filePath, cleanup := createTempFile(b)
	defer cleanup()

	fw := NewFileWriter(
		collectorDomain.LogPath(filePath),
		collectorDomain.BufferSize(8192),
		0,
		logger,
	)

	ctx := context.Background()
	err := fw.connect(ctx)
	if err != nil {
		b.Fatalf("connect failed: %v", err)
	}
	defer func() {
		_ = fw.Close()
	}()

	fw.setReady(true)
	testData := []byte("benchmark test data line\n")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			err := fw.Write(testData)
			if err != nil {
				b.Errorf("Write failed: %v", err)
			}
		}
	})
}
