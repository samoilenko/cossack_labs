// Collector is a sensor data collection service that receives sensor data.
//
// Usage: collector -bind-address=:8081  -rate-limit=33 -buffer-size=100 -flush-interval=1s -log-file=./data.txt
//
// Required flags:
//
//	-bind-address: server bind address (e.g., :8081 or 0.0.0.0:8081)
//	-log-file: path to the output log file
//	-buffer-size: buffer size in bytes for file writing
//	-flush-interval: how often to flush the buffer to disk
//	-rate-limit: maximum number of messages per second to process
package main

import (
	"context"
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
	collectorDomain "github.com/samoilenko/cossack_labs/collector/domain"
	collectorInfrastructure "github.com/samoilenko/cossack_labs/collector/infrastructure"
	gen "github.com/samoilenko/cossack_labs/pkg/sensorpb/v1"
	sensorConnect "github.com/samoilenko/cossack_labs/pkg/sensorpb/v1/sensorpbv1connect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func endWithError(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
	flag.Usage()
	os.Exit(1)
}

func main() {
	ctx, finish := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer finish()

	config, err := collectorInfrastructure.GetFromCommandLineParameters()
	if err != nil {
		endWithError(err)
	}

	// create instances)
	stdlibLogger := log.New(os.Stdout, "", log.LstdFlags)
	logger := collectorDomain.NewStdLogger(stdlibLogger)

	logger.Info("Creating services...")
	wg := &sync.WaitGroup{}
	writer := collectorInfrastructure.NewFileWriter(
		config.LogPath,
		config.BufferSize,
		config.FlushInterval,
		logger,
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		writer.Start(ctx)
	}()

	// configure on a message interceptors
	sensorDataValidator := collectorInfrastructure.NewSensorDataValidator()
	rateLimiter := collectorInfrastructure.NewRateLimiter[*gen.SensorData](config.RateLimit, 1*time.Second)
	interceptors := collectorDomain.WithInterceptors[gen.SensorData](sensorDataValidator, rateLimiter)

	streamConsumer := collectorInfrastructure.NewStreamConsumer(interceptors, logger)

	sensorDataCollector := collectorDomain.NewSensorDataCollector(logger, writer)
	wg.Add(1)
	go func() {
		defer wg.Done()
		sensorDataCollector.Start(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		sensorDataCollector.Consume(ctx, streamConsumer.GetReadDataChannel())
		logger.Info("Sensor data collector stopped")
	}()

	handlerInterceptors := connect.WithInterceptors(
		collectorInfrastructure.NewPanicRecoveryInterceptor(logger),
	)

	sensor, sensorHandler := sensorConnect.NewSensorServiceHandler(streamConsumer, handlerInterceptors)
	mux := http.NewServeMux()
	mux.Handle(sensor, sensorHandler)

	server := &http.Server{
		Addr:        string(config.BindAddress),
		Handler:     h2c.NewHandler(mux, &http2.Server{}),
		BaseContext: func(net.Listener) context.Context { return ctx },
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		logger.Info("Shutting down GRPC server...")
		if err := server.Shutdown(ctx); err != nil {
			logger.Error("shutdown error: %s", err.Error())
		}
	}()

	logger.Info("Listening on %s\n", config.BindAddress)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("listenAndServe error: %s", err.Error())
		finish()
	}

	logger.Info("closing data channel...")
	streamConsumer.Stop()

	wg.Wait()
	logger.Info("All components stopped gracefully")
}
