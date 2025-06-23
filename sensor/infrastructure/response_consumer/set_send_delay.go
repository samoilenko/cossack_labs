package responseconsumer

import (
	"context"
	"sync"
	"time"

	sensorpb "github.com/samoilenko/cossack_labs/pkg/sensorpb/v1"
	sensorInfrastructure "github.com/samoilenko/cossack_labs/sensor/infrastructure"
)

// RetryDelayConsumer creates a response consumer that manages retry delay periods
// based on server resource exhaustion responses.
func RetryDelayConsumer(
	ctx context.Context,
	retryAfterDelay *sensorInfrastructure.RetryAfterDelay,
) (chan<- *sensorpb.Response, <-chan struct{}) {
	wg := sync.WaitGroup{}

	respCh := make(chan *sensorpb.Response, 10)
	done := make(chan struct{})
	var cancel func()
	defer func() {
		if cancel != nil {
			cancel()
		}
	}()
	go func() {
		for resp := range respCh {
			switch resp.Code {
			case sensorpb.Codes_CODE_RESOURCE_EXHAUSTED:
				duration := resp.RetryAfter.AsDuration()
				retryAfterDelay.Set(duration)
				if cancel != nil {
					cancel()
				}

				ctx, newCancel := context.WithCancel(ctx)
				cancel = newCancel
				wg.Add(1)
				go func() {
					defer wg.Done()
					resetDelayAfterDuration(ctx, retryAfterDelay, duration)
				}()
			}
		}
		wg.Wait()
		close(done)
	}()

	return respCh, done
}

func resetDelayAfterDuration(
	ctx context.Context,
	retryAfterDelay *sensorInfrastructure.RetryAfterDelay,
	duration time.Duration,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(duration):
			retryAfterDelay.Reset()
		}
	}
}
