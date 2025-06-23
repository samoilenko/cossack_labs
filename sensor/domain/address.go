package domain

import (
	"errors"
	"net/url"
)

// Address of the telemetry sink.
type Address string

// NewAddress validates the given string and returns it as a Address.
// It returns an error if address equal an empty string.
func NewAddress(address string) (Address, error) {
	if len(address) == 0 {
		return "", errors.New("address cannot be empty")
	}

	parsedURL, err := url.Parse(address)
	if err != nil {
		return "", errors.New("address must be a valid URL")
	}

	if parsedURL.Scheme == "" {
		return "", errors.New("address must include http:// or https:// scheme")
	}

	if parsedURL.Host == "" {
		return "", errors.New("address must include a valid host")
	}

	return Address(address), nil
}
