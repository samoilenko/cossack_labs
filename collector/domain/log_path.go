package domain

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// LogPath represents a validated file system path for log output.
type LogPath string

// NewLogPath creates a new LogPath.
func NewLogPath(path string) (LogPath, error) {
	if len(path) == 0 {
		return "", errors.New("log path cannot be empty")
	}

	cleanPath := filepath.Clean(path)
	if strings.ContainsAny(cleanPath, "<>:\"|?*") {
		return "", fmt.Errorf("log path contains invalid characters: %s", cleanPath)
	}

	return LogPath(cleanPath), nil
}
