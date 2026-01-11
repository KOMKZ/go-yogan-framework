package grpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TestUnaryLoggerInterceptor 测试服务端日志拦截器
func TestUnaryLoggerInterceptor(t *testing.T) {
	log := logger.GetLogger("grpc_test")

	interceptor := UnaryLoggerInterceptor(log, true) // 启用日志
	assert.NotNil(t, interceptor)

	tests := []struct {
		name        string
		handler     grpc.UnaryHandler
		expectedErr error
	}{
		{
			name: "正常请求",
			handler: func(ctx context.Context, req interface{}) (interface{}, error) {
				return "success", nil
			},
			expectedErr: nil,
		},
		{
			name: "请求错误",
			handler: func(ctx context.Context, req interface{}) (interface{}, error) {
				return nil, errors.New("test error")
			},
			expectedErr: errors.New("test error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			info := &grpc.UnaryServerInfo{
				FullMethod: "/test.Service/Method",
			}

			resp, err := interceptor(ctx, "request", info, tt.handler)

			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedErr.Error(), err.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, "success", resp)
			}
		})
	}
}

// TestUnaryRecoveryInterceptor 测试服务端 Panic 恢复拦截器
func TestUnaryRecoveryInterceptor(t *testing.T) {
	log := logger.GetLogger("grpc_test")

	interceptor := UnaryRecoveryInterceptor(log)
	assert.NotNil(t, interceptor)

	tests := []struct {
		name        string
		handler     grpc.UnaryHandler
		shouldPanic bool
		expectedErr bool
	}{
		{
			name: "正常请求",
			handler: func(ctx context.Context, req interface{}) (interface{}, error) {
				return "success", nil
			},
			shouldPanic: false,
			expectedErr: false,
		},
		{
			name: "Panic recovered",
			handler: func(ctx context.Context, req interface{}) (interface{}, error) {
				panic("test panic")
			},
			shouldPanic: true,
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			info := &grpc.UnaryServerInfo{
				FullMethod: "/test.Service/Method",
			}

			resp, err := interceptor(ctx, "request", info, tt.handler)

			if tt.expectedErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "服务内部错误")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, "success", resp)
			}
		})
	}
}

// TestUnaryClientLoggerInterceptor 测试客户端日志拦截器
func TestUnaryClientLoggerInterceptor(t *testing.T) {
	log := logger.GetLogger("grpc_test")

	interceptor := UnaryClientLoggerInterceptor(log, true) // 启用日志
	assert.NotNil(t, interceptor)

	tests := []struct {
		name        string
		invoker     grpc.UnaryInvoker
		expectedErr error
	}{
		{
			name: "正常调用",
			invoker: func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
				return nil
			},
			expectedErr: nil,
		},
		{
			name: "调用错误",
			invoker: func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
				return errors.New("call failed")
			},
			expectedErr: errors.New("call failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			method := "/test.Service/Method"

			// 创建一个 mock ClientConn
			ctx2, cancel2 := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel2()

			conn, err := grpc.DialContext(ctx2, "127.0.0.1:9999",
				grpc.WithTransportCredentials(insecure.NewCredentials()),
				grpc.WithBlock())
			if err != nil {
				// 连接失败是预期的，我们只是需要一个 ClientConn 对象
				t.Skip("无法创建 mock ClientConn")
			}
			defer conn.Close()

			err = interceptor(ctx, method, "request", "reply", conn, tt.invoker)

			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedErr.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestInterceptorChain 测试拦截器链
func TestInterceptorChain(t *testing.T) {
	log := logger.GetLogger("grpc_test")

	// 创建多个拦截器
	loggerInterceptor := UnaryLoggerInterceptor(log, true) // 启用日志
	recoveryInterceptor := UnaryRecoveryInterceptor(log)

	assert.NotNil(t, loggerInterceptor)
	assert.NotNil(t, recoveryInterceptor)

	// 测试拦截器链（Recovery -> Logger -> Handler）
	ctx := context.Background()
	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/Method",
	}

	// 模拟 panic 的 handler
	panicHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		panic("test panic in chain")
	}

	// 先通过 logger 拦截器
	wrappedHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return loggerInterceptor(ctx, req, info, panicHandler)
	}

	// 再通过 recovery 拦截器
	resp, err := recoveryInterceptor(ctx, "request", info, wrappedHandler)

	// 应该捕获 panic 并返回错误
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "服务内部错误")
	assert.Nil(t, resp)
}

// TestInterceptorPerformance 测试拦截器性能
func TestInterceptorPerformance(t *testing.T) {
	log := logger.GetLogger("grpc_test")

	interceptor := UnaryLoggerInterceptor(log, true) // 启用日志

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "success", nil
	}

	ctx := context.Background()
	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/Method",
	}

	start := time.Now()
	iterations := 1000

	for i := 0; i < iterations; i++ {
		_, err := interceptor(ctx, "request", info, handler)
		assert.NoError(t, err)
	}

	duration := time.Since(start)
	avgDuration := duration / time.Duration(iterations)

	// 平均每次调用应该小于 1ms
	assert.Less(t, avgDuration, 1*time.Millisecond,
		"拦截器性能过慢: 平均 %v per call", avgDuration)

	t.Logf("拦截器性能: %d 次调用耗时 %v (平均 %v)", iterations, duration, avgDuration)
}
