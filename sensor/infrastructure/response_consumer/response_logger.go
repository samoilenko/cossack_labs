package response_consumer

import (
	sensorpb "github.com/samoilenko/cossack_labs/pkg/sensorpb/v1"
	sensorDomain "github.com/samoilenko/cossack_labs/sensor/domain"
)

func ResponseLogger(logger sensorDomain.Logger) (chan<- *sensorpb.Response, <-chan struct{}) {
	dataCh := make(chan *sensorpb.Response, 10)
	done := make(chan struct{})
	go func() {
		for resp := range dataCh {
			switch resp.Code {
			case sensorpb.Codes_CODE_RESOURCE_EXHAUSTED:
				logger.Info("message dropped, retry after delay: %s, \t correlationId: %d",
					resp.RetryAfter.AsDuration(), resp.CorrelationId,
				)
			case sensorpb.Codes_CODE_INVALID_ARGUMENT, sensorpb.Codes_CODE_INTERNAL:
				logger.Error("server responded with error code %s: %s, \t correlationId: %d",
					resp.Code.String(), resp.Message, resp.CorrelationId,
				)
			}
		}
		close(done)
	}()

	return dataCh, done
}
