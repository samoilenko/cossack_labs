FROM golang:1.24-alpine AS builder
RUN apk --no-cache add ca-certificates git
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY collector/ collector/
COPY pkg/ pkg/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -a -installsuffix cgo -ldflags="-s -w" -o collector ./collector

FROM alpine:3.18
RUN apk --no-cache add ca-certificates
RUN addgroup -S app && adduser -S app -G app
WORKDIR /home/app
USER app

COPY --from=builder /app/collector .

ENTRYPOINT ["sh", "-c"]
CMD ["./collector -bind-address=\"${COLLECTOR_BIND_ADDRESS}\" -rate-limit=\"${COLLECTOR_RATE_LIMIT}\" -buffer-size=\"${COLLECTOR_BUFFER_SIZE}\" -flush-interval=\"${COLLECTOR_FLUSH_INTERVAL}\" -log-file=\"${COLLECTOR_LOG_FILE}\""]
