package infrastructure

import "errors"

type Config struct {
	SensorName  string
	Rate        uint
	SinkAddress string
}

func NewConfig(sensorName string, rate int, sinkAddress string) (*Config, error) {
	if sensorName == "" {
		return nil, errors.New("sensor name must be non empty string")
	}

	if rate <= 0 {
		return nil, errors.New("rate must be a number greater than zero")
	}

	if sinkAddress == "" { // omit validation of address correctness
		return nil, errors.New("sink address must be non empty string")
	}

	config := &Config{
		SensorName:  sensorName,
		Rate:        uint(rate),
		SinkAddress: sinkAddress,
	}
	return config, nil
}
