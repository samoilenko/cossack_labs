package domain

import "errors"

// Address of the telemetry sink.
type Address string

// NewAddress validates the given string and returns it as a Address.
// It returns an error if address equal an empty string.
func NewAddress(address string) (Address, error) {
	if len(address) == 0 {
		return "", errors.New("address cannot be empty")
	}
	return Address(address), nil
}
