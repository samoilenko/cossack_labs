package infrastructure

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	collectorDomain "github.com/samoilenko/cossack_labs/collector/domain"
)

// PanicRecoveryInterceptor is a Connect interceptor that catches panics in gRPC handlers
// and converts them to proper error responses instead of crashing the server.
type PanicRecoveryInterceptor struct {
	logger collectorDomain.Logger
}

// WrapUnary wraps unary (request-response) gRPC handlers with panic recovery.
// If a panic occurs during handler execution, it logs the panic and returns
// an internal server error to the client instead of crashing the server.
func (i *PanicRecoveryInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return connect.UnaryFunc(func(
		ctx context.Context,
		req connect.AnyRequest,
	) (resp connect.AnyResponse, err error) {
		defer func() {
			if rec := recover(); rec != nil {
				i.logger.Error(
					"panic in unary handler %s. Panic: %v",
					req.Spec().Procedure,
					rec,
				)

				resp = nil
				err = connect.NewError(connect.CodeInternal, errors.New("internal server error"))
			}
		}()

		return next(ctx, req)
	})
}

// WrapStreamingClient wraps streaming client connections.
func (i *PanicRecoveryInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return connect.StreamingClientFunc(func(
		ctx context.Context,
		spec connect.Spec,
	) connect.StreamingClientConn {
		return next(ctx, spec)
	})
}

// WrapStreamingHandler wraps streaming gRPC handlers with panic recovery.
// If a panic occurs during streaming handler execution, it logs the panic
// and returns an internal server error instead of crashing the server.
func (i *PanicRecoveryInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return connect.StreamingHandlerFunc(func(
		ctx context.Context,
		conn connect.StreamingHandlerConn,
	) (err error) {
		defer func() {
			if rec := recover(); rec != nil {
				i.logger.Error(
					"panic in streaming handler %s. Panic: %v",
					conn.Spec().Procedure,
					rec,
				)

				err = connect.NewError(connect.CodeInternal, errors.New("internal server error"))
			}
		}()

		return next(ctx, conn)
	})
}

// NewPanicRecoveryInterceptor creates a new instance of PanicRecoveryInterceptor.
func NewPanicRecoveryInterceptor(logger collectorDomain.Logger) *PanicRecoveryInterceptor {
	return &PanicRecoveryInterceptor{
		logger: logger,
	}
}
