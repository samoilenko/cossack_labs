/**
 * This component reads data from a sensor and transfers data to a consumer
 *
 * It receives the following parameters:
 * 	1. Rate - number of messages per second to send.
 * 	2  Sensor name - the name of the sensor to use.
 * 	3. Address of the telemetry sink.
 *
 * Usage example: sensor -rate 5 -name TEMP2 -address=http://consumer.com:8080
 */
package main

import (
	"flag"
	"fmt"
	"os"

	sensorDomain "github.com/samoilenko/cossack_labs/sensor/domain"
)

func endWithError(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
	flag.Usage()
	os.Exit(1)
}

func main() {
	rawSensorName := flag.String("name", "", " sensor name")
	rawSinkAddress := flag.String("address", "", " address of the telemetry sink")
	rawRate := flag.Int("rate", 0, "number of messages per second to send, greater than 0")

	flag.Parse()
	if *rawSensorName == "" || *rawSinkAddress == "" || *rawRate <= 0 {
		fmt.Fprintln(os.Stderr, "Error: missing or invalid flags")
		flag.Usage()
		os.Exit(1)
	}

	address, err := sensorDomain.NewAddress(*rawSinkAddress)
	if err != nil {
		endWithError(err)
	}

	rate, err := sensorDomain.NewRate(*rawRate)
	if err != nil {
		endWithError(err)
	}

	sensorName, err := sensorDomain.NewSensorName(*rawSensorName)
	if err != nil {
		endWithError(err)
	}
}
