# cossack_labs

## TL;DR
For quick set up, use these env varibales:

SENSOR_COLLECTOR_ADDRESS=http://collector:8081

SENSOR_NAME=Water-bath

SENSOR_RATE=2

COLLECTOR_BIND_ADDRESS=:8081

COLLECTOR_RATE_LIMIT=33

COLLECTOR_BUFFER_SIZE=100

COLLECTOR_FLUSH_INTERVAL=5s

COLLECTOR_LOG_FILE=./data.txt


docker composer up --build -d

## Description
Application implements two behaviors:
 - sensor emulator
 - data collector from a sensor

All services responsible for I/O communication tries to establish connection if one looses.

### Code structure
 - /collector - code related to the telemetry sink component
 - /sensor - code related to the sensor component
 - /proto - proto file descibed contract between sensor component and collector component
 - /pkg - generated code from the proto file
 - docker-compose.yml and /docker - docker files to run the project
 - .env.example - allowed ENV variables
 - buf.gen.yaml and buf.yaml - Buf configs
 - go.mod go.sum - the project dependencies

### Tools 
 - Linter is added as a tool, and can be run `go tool revive ./...`
 - Tests `go test -race ./...`
 - generating code from proto files `docker run --volume "$(pwd):/workspace" --workdir /workspace bufbuild/buf generate --debug`

### Sensor emulator
It worsk in the following way: read data from sensor, tries to send data to the collector. If delivery is unsuccessfull than the message is dropped.

This component has several service
1) sensor data generator. It generates arbitrary number. Implementation `sensor/infrastructure/dummy_sensor.go`
2) sensor data reader. It read data from the sensor in the configured rate.
   Reading interval calculated as time.Second / time.Duration(rate)
   implementation `sensor/domain/value_reader.go`
3) sensor data sender. It receves data from reader(using channel) and expand data with correlation id and sensor name then sends data to the collector using configured transport(gRPC stream).

   Each message is sent to collector with timeout (3 seconds).

   If collector returns rate limit error than sender stop sending message for period provided by collector.

   If transport returns connection error, then sending process holds on to 1 second.

   In any other cases the error just logged

   All undelivered messages just are dropped

   Each sending wrapped with recovery panic (implementation `sensor/domain/safe_fn_run.go`)

   Implementation(`sensor/domain/sender.go`)
   
5) sensor data transport. It delivers messages to the collector, logs errors received from the collector. If errors occured during sending they will converted to domain errors and passed to the sender layer. Implementation `sensor/infrastructure/grpc_stream_sender.go`
6) Usage example: sensor -rate 5 -name TEMP2 -address=http://consumer.com:8080

### Collector (A telemetry sink)
The component get data from sensors, validate incoming data, uses one rate limiter for all incoming connections and save data in the file.

This componen has the following services:
1) stream consumer. It receives data from the stream, runs interceptors against sensor data, process message and inform a sender about errors. Implementation `collector/infrastructure/stream_consumer.go`
2) message interceptors:
  - Data validation. It validates that time is not too old, sensor name has less than 10 characters. Implementation `collector/infrastructure/sensor_data_validator.go`
  - Rate limiter. It works as a fixed window, if limit is exseeded, returns `RateLimitError`. Implementation `collector/infrastructure/rate_limiter.go`
3) sensor data collector. It gets data from the stream consumer using channel, convert data to log line and pass it to the writer service. Implementation `collector/domain/sensor_data_collector.go`
4) data writer. It buffers data and flush it to the disk when buffer is full or flush interval occurrs. Implementation `collector/infrastructure/file_writer.go`
5) gRPC panic recovery interceptor. Implementation `collector/infrastructure/panic_recovery_interceptor.go`
6) Usage: collector -bind-address=:8081  -rate-limit=33 -buffer-size=100 -flush-interval=1s -log-file=./data.txt

### Room for improvements
- [ ] Add error channel
