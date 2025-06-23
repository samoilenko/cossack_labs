// Package responseconsumer provides utilities for broadcasting data from a single
// source channel to multiple consumer channels.
package responseconsumer

// BroadcastResponses reads data from a source channel and broadcasts it to multiple consumer channels
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
