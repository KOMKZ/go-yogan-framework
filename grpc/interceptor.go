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

// UnaryLoggerInterceptor 服务端日志拦截器（支持配置开关）
func UnaryLoggerInterceptor(log *logger.CtxZapLogger, enableLog bool) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(start)

		// 只有启用日志时才记录
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

// UnaryRecoveryInterceptor 服务端 Panic 恢复拦截器
func UnaryRecoveryInterceptor(log *logger.CtxZapLogger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.ErrorCtx(ctx, "gRPC panic recovered",
					zap.String("method", info.FullMethod),
					zap.Any("panic", r),
				)
				err = fmt.Errorf("服务内部错误")
			}
		}()
		return handler(ctx, req)
	}
}

// UnaryClientLoggerInterceptor 客户端日志拦截器（支持配置开关）
func UnaryClientLoggerInterceptor(log *logger.CtxZapLogger, enableLog bool) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{},
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		start := time.Now()
		err := invoker(ctx, method, req, reply, cc, opts...)
		duration := time.Since(start)

		// 只有启用日志时才记录
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

// UnaryClientTimeoutInterceptor 客户端超时拦截器
// 自动为每个 RPC 调用添加超时控制
func UnaryClientTimeoutInterceptor(timeout time.Duration, log *logger.CtxZapLogger) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{},
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {

		// 如果 context 已经有 deadline，使用更小的那个
		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}

		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// UnaryClientBreakerInterceptor 客户端熔断器拦截器
// 注意：clientMgr 用于动态获取 breaker，因为 breaker 在 Component.Start() 时才注入
func UnaryClientBreakerInterceptor(clientMgr *ClientManager, serviceName string) grpc.UnaryClientInterceptor {
	// 创建专用 logger
	log := logger.GetLogger("breaker-interceptor")

	return func(ctx context.Context, method string, req, reply interface{},
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {

		// 获取熔断器
		breakerMgr := clientMgr.GetBreaker()
		if breakerMgr == nil {
			// 熔断器未启用，直接调用
			log.DebugCtx(ctx, "🔍 [Breaker] Not enabled, calling directly",
				zap.String("service", serviceName),
				zap.String("method", method))
			return invoker(ctx, method, req, reply, cc, opts...)
		}

		log.DebugCtx(ctx, "🔍 [Breaker] Interceptor executing",
			zap.String("service", serviceName),
			zap.String("method", method))

		// 包装调用为 breaker.Request（使用服务名作为 resource）
		breakerReq := &breaker.Request{
			Resource: serviceName, // 服务级熔断
			Execute: func(execCtx context.Context) (interface{}, error) {
				err := invoker(execCtx, method, req, reply, cc, opts...)
				log.DebugCtx(ctx, "🔍 [Breaker] Actual call completed",
					zap.String("service", serviceName),
					zap.Error(err))
				return reply, err
			},
		}

		// 通过熔断器执行
		log.DebugCtx(ctx, "🔍 [Breaker] Preparing to execute circuit breaker", zap.String("service", serviceName))
		_, err := breakerMgr.Execute(ctx, breakerReq)
		log.DebugCtx(ctx, "🔍 [Breaker] Circuit breaker execution completed",
			zap.String("service", serviceName),
			zap.Error(err))
		return err
	}
}
