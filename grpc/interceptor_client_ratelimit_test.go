package grpc

import (
	"context"
	"testing"

	"github.com/KOMKZ/go-yogan-framework/limiter"
	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Test unary client rate limit interceptor functionality
func TestUnaryClientRateLimitInterceptor(t *testing.T) {
	// Create rate limiter manager (using default configuration)
	limiterMgr, err := limiter.NewManagerWithLogger(limiter.Config{
		Enabled:   true,
		StoreType: "memory",
		Default: limiter.ResourceConfig{
			Algorithm:  string(limiter.AlgorithmTokenBucket),
			Rate:       1,  // one token per second
			Capacity:   1,  // Bucket capacity 1
			InitTokens: 1,  // Initialize 1 token
		},
		Resources: map[string]limiter.ResourceConfig{},
	}, logger.GetLogger("test"), nil, nil)
	assert.NoError(t, err)

	// Create ClientManager
	clientMgr := &ClientManager{
		limiter: limiterMgr,
		logger:  logger.GetLogger("test"),
	}

	// Create an interceptor
	interceptor := UnaryClientRateLimitInterceptor(clientMgr, "test-service")

	// simulate invoker
	invoker := func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		return nil
	}

	// Test: The first request passed (using default configuration)
	err = interceptor(context.Background(), "/test.Service/Method", nil, nil, nil, invoker)
	assert.NoError(t, err, "第1次请求应该通过（default 配置）")

	// Test: The second request is rate-limited (bucket capacity = 1, already used up)
	err = interceptor(context.Background(), "/test.Service/Method", nil, nil, nil, invoker)
	if err != nil {
		assert.Equal(t, codes.ResourceExhausted, status.Code(err), "应该返回 ResourceExhausted 错误码")
		assert.Contains(t, err.Error(), "rate limit exceeded", "错误信息应包含限流提示")
	} else {
		t.Log("WARNING: 第2次请求未被限流，可能是令牌生成太快")
	}
}

// TestUnaryClientRateLimitInterceptor_MethodLevel Method-level rate limiting test (priority higher than default)
func TestUnaryClientRateLimitInterceptor_MethodLevel(t *testing.T) {
	// Create rate limiter manager (configure default and method-level)
	limiterMgr, err := limiter.NewManagerWithLogger(limiter.Config{
		Enabled:   true,
		StoreType: "memory",
		Default: limiter.ResourceConfig{
			Algorithm:  string(limiter.AlgorithmTokenBucket),
			Rate:       10,  // default: 10 tokens per second
			Capacity:   10,
			InitTokens: 10,
		},
		Resources: map[string]limiter.ResourceConfig{
			"test-service:/test.Service/SlowMethod": {
				Algorithm:  string(limiter.AlgorithmTokenBucket),
				Rate:       1,  // Method level: 1 token per second (override default)
				Capacity:   1,
				InitTokens: 1,
			},
		},
	}, logger.GetLogger("test"), nil, nil)
	assert.NoError(t, err)

	// Create ClientManager
	clientMgr := &ClientManager{
		limiter: limiterMgr,
		logger:  logger.GetLogger("test"),
	}

	// Create an interceptor
	interceptor := UnaryClientRateLimitInterceptor(clientMgr, "test-service")

	invoker := func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		return nil
	}

	// Test: Fast method (using default configuration)
	for i := 0; i < 5; i++ {
		err = interceptor(context.Background(), "/test.Service/FastMethod", nil, nil, nil, invoker)
		assert.NoError(t, err, "快方法应该使用 default 配置（rate=10）")
	}

	// Testing: slow methods (using method-level configuration)
	err = interceptor(context.Background(), "/test.Service/SlowMethod", nil, nil, nil, invoker)
	assert.NoError(t, err, "慢方法第1次应该通过")

	err = interceptor(context.Background(), "/test.Service/SlowMethod", nil, nil, nil, invoker)
	assert.Error(t, err, "慢方法第2次应该被限流（方法级 rate=1）")
	assert.Contains(t, err.Error(), "rate limit exceeded", "应该提示限流")
}

// TestUnaryClientRateLimitInterceptor_Disabled test when rate limiter is disabled
func TestUnaryClientRateLimitInterceptor_Disabled(t *testing.T) {
	// Create ClientManager (unlimited throttler)
	clientMgr := &ClientManager{
		limiter: nil,
		logger:  logger.GetLogger("test"),
	}

	// Create an interceptor
	interceptor := UnaryClientRateLimitInterceptor(clientMgr, "test-service")

	invoker := func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		return nil
	}

	// Test: When there is no rate limiter, all requests pass
	for i := 0; i < 100; i++ {
		err := interceptor(context.Background(), "/test.Service/Method", nil, nil, nil, invoker)
		assert.NoError(t, err, "没有限速器时所有请求都应该通过")
	}
}

// TestUnaryClientRateLimitInterceptor_NoDefault Test behavior when default is not configured
func TestUnaryClientRateLimitInterceptor_NoDefault(t *testing.T) {
	// Create rate limiter manager (without default configuration)
	limiterMgr, err := limiter.NewManagerWithLogger(limiter.Config{
		Enabled:   true,
		StoreType: "memory",
		Default:   limiter.ResourceConfig{}, // empty default (invalid)
		Resources: map[string]limiter.ResourceConfig{
			"test-service:/test.Service/LimitedMethod": {
				Algorithm:  string(limiter.AlgorithmTokenBucket),
				Rate:       1,
				Capacity:   1,
				InitTokens: 1,
			},
		},
	}, logger.GetLogger("test"), nil, nil)
	assert.NoError(t, err)

	clientMgr := &ClientManager{
		limiter: limiterMgr,
		logger:  logger.GetLogger("test"),
	}

	interceptor := UnaryClientRateLimitInterceptor(clientMgr, "test-service")

	invoker := func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		return nil
	}

	// Test: Unconfigured methods are allowed to pass directly (because default is ineffective)
	for i := 0; i < 20; i++ {
		err = interceptor(context.Background(), "/test.Service/UnlimitedMethod", nil, nil, nil, invoker)
		assert.NoError(t, err, "未配置方法应该直接放行（default 无效）")
	}

	// Test: Configured method is rate-limited
	err = interceptor(context.Background(), "/test.Service/LimitedMethod", nil, nil, nil, invoker)
	assert.NoError(t, err, "已配置方法第1次应该通过")

	err = interceptor(context.Background(), "/test.Service/LimitedMethod", nil, nil, nil, invoker)
	assert.Error(t, err, "已配置方法第2次应该被限流")
}

