package infrastructure

import (
	"flag"

	sensorDomain "github.com/samoilenko/cossack_labs/sensor/domain"
)

// GetConfigParameters returns validated sensor configuration.
func GetConfigParameters() (sensorDomain.Address, sensorDomain.SensorName, sensorDomain.Rate, error) {
	rawSensorName := flag.String("name", "", " sensor name")
	rawSinkAddress := flag.String("address", "", " address of the telemetry sink")
	rawRate := flag.Int("rate", 0, "number of messages per second to send, greater than 0")

	flag.Parse()

	address, err := sensorDomain.NewAddress(*rawSinkAddress)
	if err != nil {
		return "", "", 0, err
	}

	rate, err := sensorDomain.NewRate(*rawRate)
	if err != nil {
		return "", "", 0, err
	}

	sensorName, err := sensorDomain.NewSensorName(*rawSensorName)
	if err != nil {
		return "", "", 0, err
	}

	return address, sensorName, rate, nil
}
