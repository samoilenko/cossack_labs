// This component reads data from a sensor and transfers data to a consumer
//
// Usage example: sensor -rate 5 -name TEMP2 -address=http://consumer.com:8080
//
// Required flags:
//
//	-rate: number of messages per second to send
//	-name: sensor name of the sensor to use
//	-address: address of the telemetry sink (e.g., http://127.0.0.1:8081)
package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"connectrpc.com/connect"
	sensorConnect "github.com/samoilenko/cossack_labs/pkg/sensorpb/v1/sensorpbv1connect"
	sensorDomain "github.com/samoilenko/cossack_labs/sensor/domain"
	sensorInfrastructure "github.com/samoilenko/cossack_labs/sensor/infrastructure"
	"golang.org/x/net/http2"
)

func endWithError(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
	flag.Usage()
	os.Exit(1)
}

func main() {
	ctx, finish := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer finish()
	rawSensorName := flag.String("name", "", " sensor name")
	rawSinkAddress := flag.String("address", "", " address of the telemetry sink")
	rawRate := flag.Int("rate", 0, "number of messages per second to send, greater than 0")

	flag.Parse()

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

	stdlibLogger := log.New(os.Stdout, "", log.LstdFlags)
	logger := sensorDomain.NewStdLogger(stdlibLogger)

	httpClient := &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLS: func(network, addr string, _ *tls.Config) (net.Conn, error) {
				return net.Dial(network, addr)
			},
		},
	}
	client := sensorConnect.NewSensorServiceClient(
		httpClient,
		string(address),
		connect.WithGRPC(),
	)

	wg := &sync.WaitGroup{}

	transport := sensorInfrastructure.NewGrpcStreamSender(client, logger)
	wg.Add(1)
	go func() {
		defer wg.Done()
		transport.Run(ctx)
	}()

	reader := sensorDomain.NewValueReader(rate, 2, logger)
	valuesCh := reader.Read(ctx, sensorInfrastructure.DummySensor{})
	sender := sensorDomain.NewSensorDataSender(transport, logger, sensorName)
	sender.Send(ctx, valuesCh)
	wg.Wait()
}
