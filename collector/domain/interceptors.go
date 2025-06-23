package domain

// Interceptor defines a contract for processing messages of type K.
// The Apply method should return an error if processing fails,
// which will halt the interceptor chain execution.
type Interceptor[K any] interface {
	Apply(msg *K) error
}

// Interceptors manages a collection of interceptors and applies them sequentially to messages.
type Interceptors[K any] struct {
	Interceptors []Interceptor[K]
}

// Apply executes all interceptors in order on the given message.
// If any interceptor returns an error, execution stops immediately
// and the error is returned, preventing subsequent interceptors from running.
func (i *Interceptors[K]) Apply(msg *K) error {
	for _, interceptor := range i.Interceptors {
		if err := interceptor.Apply(msg); err != nil {
			return err
		}
	}

	return nil
}

// WithInterceptors creates a new Interceptors instance with the provided interceptors.
// Example usage:
//
//	interceptorChain := WithInterceptors(
//	    &SensorDataValidator{},
//	)
//	err := interceptorChain.Apply(&myMessage)
func WithInterceptors[K any](interceptors ...Interceptor[K]) *Interceptors[K] {
	return &Interceptors[K]{Interceptors: interceptors}
}
