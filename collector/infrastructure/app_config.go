package infrastructure

import (
	"flag"

	collectorDomain "github.com/samoilenko/cossack_labs/collector/domain"
)

// AppConfig holds all validated configuration parameters for the collector application.
type AppConfig struct {
	BindAddress   collectorDomain.BindAddress
	BufferSize    collectorDomain.BufferSize
	FlushInterval collectorDomain.FlushInterval
	LogPath       collectorDomain.LogPath
	RateLimit     collectorDomain.RateLimit
}

// GetFromCommandLineParameters parses command-line flags and returns validated collector configuration.
// It handles the parsing of all required collector parameters from command-line arguments
// and validates them using domain types to ensure application safety.
func GetFromCommandLineParameters() (*AppConfig, error) {
	rawBindAddress := flag.String("bind-address", "", "Bind address (e.g. 0.0.0.0:8080)")
	rawLogPath := flag.String("log-file", "", "Path to output log file")
	rawBufferSize := flag.Int("buffer-size", 0, "Buffer size in bytes")
	rawFlushInterval := flag.Duration("flush-interval", 0, "Buffer flush interval in seconds")
	rawRateLimit := flag.Int("rate-limit", 0, "Rate limit in bytes/sec")

	flag.Parse()

	bindAddress, err := collectorDomain.NewBindAddress(*rawBindAddress)
	if err != nil {
		return nil, err
	}

	logPath, err := collectorDomain.NewLogPath(*rawLogPath)
	if err != nil {
		return nil, err
	}

	bufferSize, err := collectorDomain.NewBufferSize(*rawBufferSize)
	if err != nil {
		return nil, err
	}

	flushInterval, err := collectorDomain.NewFlushInterval(*rawFlushInterval)
	if err != nil {
		return nil, err
	}

	rateLimit, err := collectorDomain.NewRateLimit(*rawRateLimit)
	if err != nil {
		return nil, err
	}

	config := &AppConfig{
		BindAddress:   bindAddress,
		BufferSize:    bufferSize,
		FlushInterval: flushInterval,
		LogPath:       logPath,
		RateLimit:     rateLimit,
	}

	return config, nil
}
