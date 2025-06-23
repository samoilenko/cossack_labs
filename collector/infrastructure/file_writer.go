package infrastructure

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	collectorDomain "github.com/samoilenko/cossack_labs/collector/domain"
)

// FileWriter provides buffered file writer implementation
// that automatically flushes data at regular intervals and supports reconnection.
type FileWriter struct {
	logPath       collectorDomain.LogPath
	logger        collectorDomain.Logger
	bufferSize    collectorDomain.BufferSize
	flushInterval collectorDomain.FlushInterval
	readyLock     sync.Mutex
	f             *os.File
	w             *bufio.Writer
	wLock         sync.Mutex
	ready         bool
}

// setReady manages the writer's availability.
func (fw *FileWriter) setReady(val bool) {
	fw.readyLock.Lock()
	defer fw.readyLock.Unlock()
	fw.ready = val
}

// IsReady returns true if the file writer is ready to accept write operations.
func (fw *FileWriter) IsReady() bool {
	fw.readyLock.Lock()
	defer fw.readyLock.Unlock()
	return fw.ready
}

// Reconnect closes the current file connection and establishes a new one.
// The writer is marked as not ready during the reconnection process and
// restored to ready state upon successful completion.
func (fw *FileWriter) Reconnect(ctx context.Context) error {
	fw.setReady(false)
	defer fw.setReady(true)

	_ = fw.Close()

	return fw.connect(ctx)
}

// flush forces all buffered data to be written to the underlying file.
func (fw *FileWriter) flush() error {
	fw.logger.Info("flushing buffer")
	fw.wLock.Lock()
	defer fw.wLock.Unlock()
	return fw.w.Flush()
}

// Close cleanly shuts down the file writer by flushing any remaining buffered data and closing the file handle.
func (fw *FileWriter) Close() error {
	if fw.w != nil {
		if err := fw.flush(); err != nil {
			fw.logger.Error("error on flushing buffer: %s", err.Error())
		}
	}

	if fw.f != nil {
		if err := fw.f.Close(); err != nil {
			fw.logger.Error("error on closing file: %s", err.Error())
		}
	}
	return nil
}

// connect establishes a connection to the log file and initializes the buffered writer.
func (fw *FileWriter) connect(_ context.Context) error {
	f, w, err := fw.openBufferedFile(string(fw.logPath), fw.bufferSize)
	if err != nil {
		fw.logger.Error("error on opening file: %s", err.Error())
		return fmt.Errorf("error on opening file: %w", err)
	}
	fw.f = f
	fw.w = w

	return nil
}

// Write writes the provided data to the buffered file writer.
// Data is written to the buffer.
func (fw *FileWriter) Write(data []byte) error {
	if !fw.IsReady() {
		return fmt.Errorf("file writer is not ready")
	}
	fw.wLock.Lock()
	defer fw.wLock.Unlock()
	if _, err := fw.w.Write(data); err != nil {
		return err
	}
	return nil
}

// openBufferedFile opens a file for append operations and wraps it with a buffered writer.
// The file is created if it doesn't exist.
func (fw *FileWriter) openBufferedFile(name string, bufferSize collectorDomain.BufferSize) (*os.File, *bufio.Writer, error) {
	f, err := os.OpenFile(name, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, nil, err
	}
	w := bufio.NewWriterSize(f, int(bufferSize))
	return f, w, nil
}

// Start initializes the file writer and begins the automatic flush routine.
func (fw *FileWriter) Start(ctx context.Context) {
	defer func() {
		if err := fw.Close(); err != nil {
			fw.logger.Error("error on closing file writer: %s", err.Error())
		}
	}()
	if err := fw.connect(ctx); err != nil {
		fw.logger.Error("error on connecting: %s", err.Error())
	}
	fw.setReady(true)

	ticker := time.NewTicker(time.Duration(fw.flushInterval))
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := fw.flush(); err != nil {
				fw.logger.Error("error on flushing buffer %s", err.Error())
			}
		}
	}
}

// NewFileWriter creates a new FileWriter instance.
// The writer is initially not ready and must be started using the Start method
// to begin file operations and automatic flushing.
func NewFileWriter(logPath collectorDomain.LogPath,
	bufferSize collectorDomain.BufferSize,
	flushInterval collectorDomain.FlushInterval,
	logger collectorDomain.Logger,
) *FileWriter {
	return &FileWriter{
		logPath:       logPath,
		bufferSize:    bufferSize,
		flushInterval: flushInterval,
		logger:        logger,
		ready:         false,
	}
}
