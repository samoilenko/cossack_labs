package domain

import "errors"

// SensorName - the name of the sensor to use.
type SensorName string

// NewSensorName validates the given string and returns it as a SensorName.
// It returns an error if name equal an empty string.
func NewSensorName(name string) (SensorName, error) {
	if name == "" {
		return "", errors.New("Sensor name cannot be empty")
	}

	return SensorName(name), nil
}
