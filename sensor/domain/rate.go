package domain

import "errors"

type Rate uint32

func NewRate(rate int) (Rate, error) {
	if rate <= 0 {
		return 0, errors.New("rate must be greater than 0")
	}
	return Rate(rate), nil
}
