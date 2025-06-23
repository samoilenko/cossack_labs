package response_consumer

func BroadcastResponses[T any](sourceCh <-chan T, responseConsumers []chan<- T) {
	for data := range sourceCh {
		for _, consumer := range responseConsumers {
			select {
			case consumer <- data:
			default:
			}
		}
	}

	for _, consumer := range responseConsumers {
		close(consumer)
	}
}
