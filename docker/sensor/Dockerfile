# 1. Builder stage: cache dependencies, drop unneeded packages
FROM golang:1.24-alpine AS builder

RUN apk --no-cache add ca-certificates git

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY sensor/ sensor/
COPY pkg/ pkg/

ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -a -installsuffix cgo -ldflags="-s -w" -o sensor ./sensor


FROM alpine:3.18

RUN addgroup -S app && adduser -S app -G app


RUN apk --no-cache add ca-certificates

WORKDIR /home/app
USER app

COPY --from=builder /app/sensor .

ENTRYPOINT ["sh", "-c"]
CMD ["./sensor -address \"$SENSOR_COLLECTOR_ADDRESS\" -name \"$SENSOR_NAME\" -rate \"$SENSOR_RATE\""]
