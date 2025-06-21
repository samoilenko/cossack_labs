package domain

import "errors"

type SensorName string

func NewSensorName(name string) (SensorName, error) {
	if name == "" {
		return "", errors.New("Sensor name cannot be empty")
	}

	return SensorName(name), nil
}
