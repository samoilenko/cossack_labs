package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/samoilenko/cossack_labs/sensor/infrastructure"
)

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
func main() {
	sensorName := flag.String("name", "", " sensor name")
	sinkAddress := flag.String("address", "", " address of the telemetry sink")
	rate := flag.Int("rate", 0, "number of messages per second to send")

	flag.Parse()
	if *sensorName == "" || *sinkAddress == "" || *rate == 0 {
		fmt.Fprintln(os.Stderr, "Error: missing or invalid flags")
		flag.Usage()
		os.Exit(1)
	}
	_, err := infrastructure.NewConfig(*sensorName, *rate, *sinkAddress)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
		flag.Usage()
		os.Exit(1)
	}

}
