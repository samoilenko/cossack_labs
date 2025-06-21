package domain

import "errors"

type Address string

func NewAddress(address string) (Address, error) {
	if len(address) == 0 {
		return "", errors.New("address cannot be empty")
	}
	return Address(address), nil
}
