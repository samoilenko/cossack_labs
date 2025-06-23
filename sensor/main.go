// This component reads data from a sensor and transfers data to a consumer
//
// Usage example: sensor -rate=5 -name=TEMP2 -address=http://consumer.com:8080
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
	"time"

	"connectrpc.com/connect"
	sensorpb "github.com/samoilenko/cossack_labs/pkg/sensorpb/v1"
	sensorConnect "github.com/samoilenko/cossack_labs/pkg/sensorpb/v1/sensorpbv1connect"
	sensorDomain "github.com/samoilenko/cossack_labs/sensor/domain"
	sensorInfrastructure "github.com/samoilenko/cossack_labs/sensor/infrastructure"
	responseConsumer "github.com/samoilenko/cossack_labs/sensor/infrastructure/response_consumer"

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

	wg := &sync.WaitGroup{}

	address, sensorName, rate, err := sensorInfrastructure.GetConfigParameters()
	if err != nil {
		endWithError(err)
	}

	// create instances
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

	streamManager := sensorInfrastructure.NewGrpcStream(client, logger)
	retryAfterDelay := sensorInfrastructure.NewRetryAfterDelay()
	transport := sensorInfrastructure.NewGrpcStreamSender(streamManager, logger, retryAfterDelay)

	// set up response consumers
	responseLogger, responseLoggerDone := responseConsumer.ResponseLogger(logger)
	RetryDelayConsumer, RetryDelayConsumerDone := responseConsumer.RetryDelayConsumer(ctx, retryAfterDelay)
	responseConsumers := []chan<- *sensorpb.Response{
		responseLogger,
		RetryDelayConsumer,
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		responseConsumer.BroadcastResponses[*sensorpb.Response](transport.GetResponseChannel(), responseConsumers)
		<-responseLoggerDone
		<-RetryDelayConsumerDone
	}()
	//

	wg.Add(1)
	go func() {
		defer wg.Done()
		transport.Run(ctx)
		logger.Info("transport stopped")
	}()

	interval := time.Second / time.Duration(rate)
	reader := sensorDomain.NewValueReader(interval, 2, logger)
	valuesCh := reader.Read(ctx, sensorInfrastructure.DummySensor{})
	sender := sensorDomain.NewSensorDataSender(transport, logger, sensorName)

	// Starts read data from a sensor and sends to a consumer
	sender.Send(ctx, valuesCh)
	logger.Info("sender stopped")

	wg.Wait()
	logger.Info("All components stopped gracefully")
}
