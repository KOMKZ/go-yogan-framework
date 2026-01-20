package grpc

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UnaryClientRateLimitInterceptor client rate limiting interceptor
//
// Resource name: {serviceName}:{method} (e.g., "auth-app:/auth.AuthService/Login")
//
// Speed limit strategy:
// If rate limiting rules are configured at the method level, use the method-level rules
// If method-level configuration is not set, use the default configuration (if default is valid)
// 3. If default is invalid or not configured, allow directly
//
// Parameters:
// - clientMgr: Client manager (used to obtain rate limiting manager)
// - serviceName: service name (name configured in grpc.clients)
func UnaryClientRateLimitInterceptor(clientMgr *ClientManager, serviceName string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		// Get rate limiter manager
		limiterMgr := clientMgr.GetLimiter()
		if limiterMgr == nil || !limiterMgr.IsEnabled() {
			// No speed limit enabled, pass through directly
			return invoker(ctx, method, req, reply, cc, opts...)
		}

		// 🎯 Check rate limiting (resource name: serviceName:method)
		// If not configured at the method level, the default configuration will be used automatically (if the default is valid)
		methodResource := fmt.Sprintf("%s:%s", serviceName, method)

		allowed, err := limiterMgr.Allow(ctx, methodResource)
		if err != nil {
			// Speed limit check failed (possibly due to configuration error), log but do not block
			// This can prevent abnormalities in the rate limiting component from affecting normal calls
			clientMgr.logger.WarnCtx(ctx, "⚠️  Rate limit check failed, allowing request",
				zap.String("service", serviceName),
				zap.String("method", method),
				zap.String("resource", methodResource),
				zap.Error(err))
			return invoker(ctx, method, req, reply, cc, opts...)
		}

		if !allowed {
			// rate limiting triggered
			clientMgr.logger.WarnCtx(ctx, "🚫 Request rate limited",
				zap.String("service", serviceName),
				zap.String("method", method),
				zap.String("resource", methodResource))
			return status.Errorf(codes.ResourceExhausted,
				"rate limit exceeded for %s", method)
		}

		// Speed limit enforcement, execute request
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
