# cossack_labs

## TL;DR
docker composer up --build -d

## Description
Application implements two behaviors:
 - sensor emulator
 - data collector from a sensor
   

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
3) sensor data sender. It receves data from reader(using channel) and sends data to the collector using configured transport(GRPC stream).

   Each message is sent to collector with timeout (3 seconds).

   If collector returns rate limit error than sender stop sending message for period provided by collector.

   If transport returns connection error, then sending process holds on to 1 second.

   In any other cases the error just logged

   All undelivered messages just are dropped

   Each sending wrapped with recovery panic (implementation `sensor/domain/safe_fn_run.go`)

   Implementation(`sensor/domain/sender.go`)
   
5) sensor data transport. It delivers messages to the collector, logs errors received from the collector. If errors occured during sending they will converted to domain errors and passed to the sender layer. Implementation `sensor/infrastructure/grpc_stream_sender.go`
6) Usage example: sensor -rate 5 -name TEMP2 -address=http://consumer.com:8080

### Room for improvements
- [ ] Add error channel
