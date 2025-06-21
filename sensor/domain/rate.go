package domain

import "errors"

// Rate - number of messages per second to send.
type Rate uint32

// NewRate validates the given integer and returns it as a Rate.
// It returns an error if rate is less than or equal to 0.
func NewRate(rate int) (Rate, error) {
	if rate <= 0 {
		return 0, errors.New("rate must be greater than 0")
	}
	return Rate(rate), nil
}
