package grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/KOMKZ/go-yogan-framework/breaker"
	"github.com/KOMKZ/go-yogan-framework/logger"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// UnaryLoggerInterceptor server log interceptor (supports configuration switch)
func UnaryLoggerInterceptor(log *logger.CtxZapLogger, enableLog bool) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(start)

		// Only record when logging is enabled
		if enableLog {
			if err != nil {
				log.ErrorCtx(ctx, "gRPC request",
					zap.String("method", info.FullMethod),
					zap.Duration("duration", duration),
					zap.Error(err),
				)
			} else {
				log.InfoCtx(ctx, "gRPC request",
					zap.String("method", info.FullMethod),
					zap.Duration("duration", duration),
				)
			}
		}

		return resp, err
	}
}

// Unary Recovery Interceptor Server Side Panic Recovery Interceptor
func UnaryRecoveryInterceptor(log *logger.CtxZapLogger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.ErrorCtx(ctx, "gRPC panic recovered",
					zap.String("method", info.FullMethod),
					zap.Any("panic", r),
				)
				err = fmt.Errorf("Internal service error")
			}
		}()
		return handler(ctx, req)
	}
}

// UnaryClientLoggerInterceptor client log interceptor (supports configuration switch)
func UnaryClientLoggerInterceptor(log *logger.CtxZapLogger, enableLog bool) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{},
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		start := time.Now()
		err := invoker(ctx, method, req, reply, cc, opts...)
		duration := time.Since(start)

		// Only record when logging is enabled
		if enableLog {
			if err != nil {
				log.ErrorCtx(ctx, "gRPC call",
					zap.String("method", method),
					zap.String("target", cc.Target()),
					zap.Duration("duration", duration),
					zap.Error(err),
				)
			} else {
				log.DebugCtx(ctx, "gRPC call",
					zap.String("method", method),
					zap.String("target", cc.Target()),
					zap.Duration("duration", duration),
				)
			}
		}

		return err
	}
}

// UnaryClientTimeoutInterceptor client timeout interceptor
// Automatically add timeout control for each RPC call
func UnaryClientTimeoutInterceptor(timeout time.Duration, log *logger.CtxZapLogger) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{},
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {

		// If the context already has a deadline, use the smaller one
		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}

		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// UnaryClientBreakerInterceptor client circuit breaker interceptor
// Note: clientMgr is used to dynamically obtain the breaker, as the breaker is injected only when Component.Start() is called.
func UnaryClientBreakerInterceptor(clientMgr *ClientManager, serviceName string) grpc.UnaryClientInterceptor {
	// Create dedicated logger
	log := logger.GetLogger("yogan")

	return func(ctx context.Context, method string, req, reply interface{},
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {

		// Get circuit breaker
		breakerMgr := clientMgr.GetBreaker()
		if breakerMgr == nil {
			// circuit breaker is disabled, call directly
			log.DebugCtx(ctx, "🔍 [Breaker] Not enabled, calling directly",
				zap.String("service", serviceName),
				zap.String("method", method))
			return invoker(ctx, method, req, reply, cc, opts...)
		}

		log.DebugCtx(ctx, "🔍 [Breaker] Interceptor executing",
			zap.String("service", serviceName),
			zap.String("method", method))

		// Wrap the call as breaker.Request (using the service name as the resource)
		breakerReq := &breaker.Request{
			Resource: serviceName, // service level circuit breaker
			Execute: func(execCtx context.Context) (interface{}, error) {
				err := invoker(execCtx, method, req, reply, cc, opts...)
				log.DebugCtx(ctx, "🔍 [Breaker] Actual call completed",
					zap.String("service", serviceName),
					zap.Error(err))
				return reply, err
			},
		}

		// Execute via circuit breaker
		log.DebugCtx(ctx, "🔍 [Breaker] Preparing to execute circuit breaker", zap.String("service", serviceName))
		_, err := breakerMgr.Execute(ctx, breakerReq)
		log.DebugCtx(ctx, "🔍 [Breaker] Circuit breaker execution completed",
			zap.String("service", serviceName),
			zap.Error(err))
		return err
	}
}
