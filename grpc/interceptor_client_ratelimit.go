package grpc

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UnaryClientRateLimitInterceptor 客户端限速拦截器
//
// 资源名称：{serviceName}:{method} (如 "auth-app:/auth.AuthService/Login")
//
// 限速策略：
// 1. 如果方法级配置了限流规则，使用方法级规则
// 2. 如果方法级未配置，使用 default 配置（如果 default 有效）
// 3. 如果 default 无效或未配置，直接放行
//
// 参数：
//   - clientMgr: 客户端管理器（用于获取限速管理器）
//   - serviceName: 服务名称（在 grpc.clients 中配置的名称）
func UnaryClientRateLimitInterceptor(clientMgr *ClientManager, serviceName string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		// 获取限速管理器
		limiterMgr := clientMgr.GetLimiter()
		if limiterMgr == nil || !limiterMgr.IsEnabled() {
			// 未启用限速，直接透传
			return invoker(ctx, method, req, reply, cc, opts...)
		}

		// 🎯 检查限流（资源名称：serviceName:method）
		// 如果方法级未配置，会自动使用 default 配置（如果 default 有效）
		methodResource := fmt.Sprintf("%s:%s", serviceName, method)

		allowed, err := limiterMgr.Allow(ctx, methodResource)
		if err != nil {
			// 限速检查失败（可能是配置错误），记录日志但不阻断
			// 这样可以避免限速组件异常影响正常调用
			clientMgr.logger.WarnCtx(ctx, "⚠️  Rate limit check failed, allowing request",
				zap.String("service", serviceName),
				zap.String("method", method),
				zap.String("resource", methodResource),
				zap.Error(err))
			return invoker(ctx, method, req, reply, cc, opts...)
		}

		if !allowed {
			// 限流触发
			clientMgr.logger.WarnCtx(ctx, "🚫 Request rate limited",
				zap.String("service", serviceName),
				zap.String("method", method),
				zap.String("resource", methodResource))
			return status.Errorf(codes.ResourceExhausted,
				"rate limit exceeded for %s", method)
		}

		// 限速通过，执行请求
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
