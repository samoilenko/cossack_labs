package domain

import (
	"errors"
	"fmt"
	"net"
	"strconv"
)

// BindAddress represents a network address where a server can bind to listen
// for incoming connections. This type ensures that only valid, non-empty
// addresses are used for server configuration.
//
// Valid address formats:
//   - "localhost:8080"
//   - "127.0.0.1:8080"
//   - "0.0.0.0:8080"
//   - ":8080" (binds to all interfaces)
//   - "[::1]:8080" (IPv6)
type BindAddress string

// NewBindAddress creates a new BindAddress with basic validation to ensure
// the address is suitable for server binding operations.
func NewBindAddress(value string) (BindAddress, error) {
	if value == "" {
		return "", errors.New("bind address must be non-empty")
	}

	_, port, err := net.SplitHostPort(value)
	if err != nil {
		return "", fmt.Errorf("invalid bind address format: %w", err)
	}

	_, err = strconv.Atoi(port)
	if err != nil {
		return "", fmt.Errorf("port must be a number: %s", port)
	}

	return BindAddress(value), nil
}
