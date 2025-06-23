package domain

import (
	"fmt"
	"log"
)

// Logger defines the contract for logging operations with different severity levels.
type Logger interface {
	// Info logs an informational message with optional formatted arguments.
	Info(msg string, args ...interface{})
	// Error logs an error message with optional formatted arguments.
	Error(msg string, args ...interface{})
}

// StdLogger implements the Logger interface using Go's standard log package.
type StdLogger struct {
	logger *log.Logger
}

// Info logs an informational message with INFO prefix using the underlying standard logger.
func (l *StdLogger) Info(msg string, args ...interface{}) {
	l.logger.Printf("INFO: %s", fmt.Sprintf(msg, args...))
}

// Error logs an error message with ERROR prefix using the underlying standard logger.
func (l *StdLogger) Error(msg string, args ...interface{}) {
	l.logger.Printf("ERROR: %s", fmt.Sprintf(msg, args...))
}

// NewStdLogger creates a new StdLogger instance wrapping the provided standard logger.
func NewStdLogger(l *log.Logger) *StdLogger {
	return &StdLogger{l}
}
