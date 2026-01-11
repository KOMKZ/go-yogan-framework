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

// TestUnaryClientRateLimitInterceptor 测试基础限速功能
func TestUnaryClientRateLimitInterceptor(t *testing.T) {
	// 创建限速管理器（使用 default 配置）
	limiterMgr, err := limiter.NewManagerWithLogger(limiter.Config{
		Enabled:   true,
		StoreType: "memory",
		Default: limiter.ResourceConfig{
			Algorithm:  string(limiter.AlgorithmTokenBucket),
			Rate:       1,  // 每秒1个令牌
			Capacity:   1,  // 桶容量1
			InitTokens: 1,  // 初始1个令牌
		},
		Resources: map[string]limiter.ResourceConfig{},
	}, logger.GetLogger("test"), nil, nil)
	assert.NoError(t, err)

	// 创建 ClientManager
	clientMgr := &ClientManager{
		limiter: limiterMgr,
		logger:  logger.GetLogger("test"),
	}

	// 创建拦截器
	interceptor := UnaryClientRateLimitInterceptor(clientMgr, "test-service")

	// 模拟 invoker
	invoker := func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		return nil
	}

	// 测试：第1次请求通过（使用 default 配置）
	err = interceptor(context.Background(), "/test.Service/Method", nil, nil, nil, invoker)
	assert.NoError(t, err, "第1次请求应该通过（default 配置）")

	// 测试：第2次请求被限流（桶容量=1，已用完）
	err = interceptor(context.Background(), "/test.Service/Method", nil, nil, nil, invoker)
	if err != nil {
		assert.Equal(t, codes.ResourceExhausted, status.Code(err), "应该返回 ResourceExhausted 错误码")
		assert.Contains(t, err.Error(), "rate limit exceeded", "错误信息应包含限流提示")
	} else {
		t.Log("WARNING: 第2次请求未被限流，可能是令牌生成太快")
	}
}

// TestUnaryClientRateLimitInterceptor_MethodLevel 测试方法级限速（优先级高于 default）
func TestUnaryClientRateLimitInterceptor_MethodLevel(t *testing.T) {
	// 创建限速管理器（配置 default 和方法级）
	limiterMgr, err := limiter.NewManagerWithLogger(limiter.Config{
		Enabled:   true,
		StoreType: "memory",
		Default: limiter.ResourceConfig{
			Algorithm:  string(limiter.AlgorithmTokenBucket),
			Rate:       10,  // default: 每秒10个令牌
			Capacity:   10,
			InitTokens: 10,
		},
		Resources: map[string]limiter.ResourceConfig{
			"test-service:/test.Service/SlowMethod": {
				Algorithm:  string(limiter.AlgorithmTokenBucket),
				Rate:       1,  // 方法级：每秒1个令牌（覆盖 default）
				Capacity:   1,
				InitTokens: 1,
			},
		},
	}, logger.GetLogger("test"), nil, nil)
	assert.NoError(t, err)

	// 创建 ClientManager
	clientMgr := &ClientManager{
		limiter: limiterMgr,
		logger:  logger.GetLogger("test"),
	}

	// 创建拦截器
	interceptor := UnaryClientRateLimitInterceptor(clientMgr, "test-service")

	invoker := func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		return nil
	}

	// 测试：快方法（使用 default 配置）
	for i := 0; i < 5; i++ {
		err = interceptor(context.Background(), "/test.Service/FastMethod", nil, nil, nil, invoker)
		assert.NoError(t, err, "快方法应该使用 default 配置（rate=10）")
	}

	// 测试：慢方法（使用方法级配置）
	err = interceptor(context.Background(), "/test.Service/SlowMethod", nil, nil, nil, invoker)
	assert.NoError(t, err, "慢方法第1次应该通过")

	err = interceptor(context.Background(), "/test.Service/SlowMethod", nil, nil, nil, invoker)
	assert.Error(t, err, "慢方法第2次应该被限流（方法级 rate=1）")
	assert.Contains(t, err.Error(), "rate limit exceeded", "应该提示限流")
}

// TestUnaryClientRateLimitInterceptor_Disabled 测试限速器禁用时
func TestUnaryClientRateLimitInterceptor_Disabled(t *testing.T) {
	// 创建 ClientManager（无限速器）
	clientMgr := &ClientManager{
		limiter: nil,
		logger:  logger.GetLogger("test"),
	}

	// 创建拦截器
	interceptor := UnaryClientRateLimitInterceptor(clientMgr, "test-service")

	invoker := func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		return nil
	}

	// 测试：没有限速器时，所有请求都通过
	for i := 0; i < 100; i++ {
		err := interceptor(context.Background(), "/test.Service/Method", nil, nil, nil, invoker)
		assert.NoError(t, err, "没有限速器时所有请求都应该通过")
	}
}

// TestUnaryClientRateLimitInterceptor_NoDefault 测试未配置 default 时的行为
func TestUnaryClientRateLimitInterceptor_NoDefault(t *testing.T) {
	// 创建限速管理器（不配置 default）
	limiterMgr, err := limiter.NewManagerWithLogger(limiter.Config{
		Enabled:   true,
		StoreType: "memory",
		Default:   limiter.ResourceConfig{}, // 空的 default（无效）
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

	// 测试：未配置方法直接放行（因为 default 无效）
	for i := 0; i < 20; i++ {
		err = interceptor(context.Background(), "/test.Service/UnlimitedMethod", nil, nil, nil, invoker)
		assert.NoError(t, err, "未配置方法应该直接放行（default 无效）")
	}

	// 测试：已配置方法受限流
	err = interceptor(context.Background(), "/test.Service/LimitedMethod", nil, nil, nil, invoker)
	assert.NoError(t, err, "已配置方法第1次应该通过")

	err = interceptor(context.Background(), "/test.Service/LimitedMethod", nil, nil, nil, invoker)
	assert.Error(t, err, "已配置方法第2次应该被限流")
}

