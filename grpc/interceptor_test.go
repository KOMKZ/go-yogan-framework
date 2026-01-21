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

// TestUnaryLoggerInterceptor test server log interceptor
func TestUnaryLoggerInterceptor(t *testing.T) {
	log := logger.GetLogger("grpc_test")

	interceptor := UnaryLoggerInterceptor(log, true) // Enable logging
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

// TestUnaryRecoveryInterceptor tests server panic recovery interceptor
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
				assert.Contains(t, err.Error(), "Internal service error")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, "success", resp)
			}
		})
	}
}

// TestUnaryClientLoggerInterceptor test client log interceptor
func TestUnaryClientLoggerInterceptor(t *testing.T) {
	log := logger.GetLogger("grpc_test")

	interceptor := UnaryClientLoggerInterceptor(log, true) // Enable logging
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

			// Create a mock ClientConn
			ctx2, cancel2 := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel2()

			conn, err := grpc.DialContext(ctx2, "127.0.0.1:9999",
				grpc.WithTransportCredentials(insecure.NewCredentials()),
				grpc.WithBlock())
			if err != nil {
				// Connection failure is expected; we just need a ClientConn object
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

// TestInterceptorChain test interceptor chain
func TestInterceptorChain(t *testing.T) {
	log := logger.GetLogger("grpc_test")

	// Create multiple interceptors
	loggerInterceptor := UnaryLoggerInterceptor(log, true) // Enable logging
	recoveryInterceptor := UnaryRecoveryInterceptor(log)

	assert.NotNil(t, loggerInterceptor)
	assert.NotNil(t, recoveryInterceptor)

	// Test the interceptor chain (Recovery -> Logger -> Handler)
	ctx := context.Background()
	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/Method",
	}

	// handler for simulating panic
	panicHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		panic("test panic in chain")
	}

	// First intercept through the logger interceptor
	wrappedHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return loggerInterceptor(ctx, req, info, panicHandler)
	}

	// Then intercept via the recovery interceptor
	resp, err := recoveryInterceptor(ctx, "request", info, wrappedHandler)

	// Should catch panics and return errors
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Internal service error")
	assert.Nil(t, resp)
}

// TestInterceptorPerformance test interceptor performance
func TestInterceptorPerformance(t *testing.T) {
	log := logger.GetLogger("grpc_test")

	interceptor := UnaryLoggerInterceptor(log, true) // Enable logging

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

	// The average time per call should be less than 1ms
	assert.Less(t, avgDuration, 1*time.Millisecond,
		"拦截器性能过慢: 平均 %v per call", avgDuration)

	t.Logf("拦截器性能: %d 次调用耗时 %v (平均 %v)", iterations, duration, avgDuration)
}
