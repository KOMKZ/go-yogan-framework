package grpc

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

// TestClientManager_Timeout_ShortTimeout Test short timeout configuration
func TestClientManager_Timeout_ShortTimeout(t *testing.T) {
	log := logger.GetLogger("grpc_test")

	// Configure an unreachable address with a 1-second timeout
	configs := map[string]ClientConfig{
		"slow-service": {
			Target:  "127.0.0.1:19999", // Service does not exist
			Timeout: 1,                 // 1 second timeout
		},
	}

	manager := NewClientManager(configs, log)
	require.NotNil(t, manager)

	// Record start time
	start := time.Now()

	// Try to get connection (should fail within 1 second)
	conn, err := manager.GetConn("slow-service")

	// Calculate duration
	elapsed := time.Since(start)

	// Verify: Should fail and take approximately 1 second
	assert.Error(t, err, "应该超时失败")
	assert.Nil(t, conn, "连接应该为空")
	assert.Less(t, elapsed, 2*time.Second, "应该在2秒内超时")
	assert.Greater(t, elapsed, 500*time.Millisecond, "应该至少等待500ms")

	t.Logf("超时测试完成，耗时: %v", elapsed)
}

// TestClientManager_Timeout_LongTimeout_Test long timeout configuration
func TestClientManager_Timeout_LongTimeout(t *testing.T) {
	log := logger.GetLogger("grpc_test")

	// Configure an unreachable address with a 10-second timeout
	configs := map[string]ClientConfig{
		"slow-service": {
			Target:  "127.0.0.1:19998", // Service does not exist
			Timeout: 10,                // 10 second timeout
		},
	}

	manager := NewClientManager(configs, log)
	require.NotNil(t, manager)

	// Record start time
	start := time.Now()

	// Try to get connection (should fail within 10 seconds)
	conn, err := manager.GetConn("slow-service")

	// Calculate duration
	elapsed := time.Since(start)

	// Validate: Should fail and take approximately 10 seconds
	assert.Error(t, err, "应该超时失败")
	assert.Nil(t, conn, "连接应该为空")
	assert.Less(t, elapsed, 12*time.Second, "应该在12秒内超时")

	t.Logf("超时测试完成，耗时: %v", elapsed)
}

// TestClientManager_Timeout_DefaultTimeout Test default timeout (when not configured)
func TestClientManager_Timeout_DefaultTimeout(t *testing.T) {
	log := logger.GetLogger("grpc_test")

	// Without configuring timeout, the default 5 seconds should be used.
	configs := map[string]ClientConfig{
		"default-timeout-service": {
			Target: "127.0.0.1:19997", // Service does not exist
			// Timeout not set, should default to 5 seconds
		},
	}

	manager := NewClientManager(configs, log)
	require.NotNil(t, manager)

	// Record start time
	start := time.Now()

	// Try to obtain connection (should fail within 5 seconds)
	conn, err := manager.GetConn("default-timeout-service")

	// Calculate time consumption
	elapsed := time.Since(start)

	// Verify: Should fail and take approximately 5 seconds
	assert.Error(t, err, "应该超时失败")
	assert.Nil(t, conn, "连接应该为空")
	assert.Less(t, elapsed, 7*time.Second, "应该在7秒内超时")
	assert.Greater(t, elapsed, 3*time.Second, "应该至少等待3秒")

	t.Logf("默认超时测试完成，耗时: %v", elapsed)
}

// TestClientManager_Timeout_DifferentTimeouts Test multiple clients with different timeout configurations
func TestClientManager_Timeout_DifferentTimeouts(t *testing.T) {
	log := logger.GetLogger("grpc_test")

	// Start a normal test server
	server := startTestGRPCServer(t)
	defer server.Stop(context.Background())

	// Configure multiple clients, different timeouts
	configs := map[string]ClientConfig{
		"fast-service": {
			Target:  fmt.Sprintf("127.0.0.1:%d", server.Port),
			Timeout: 1, // 1 second timeout (but the service is functioning normally and can connect quickly)
		},
		"normal-service": {
			Target:  fmt.Sprintf("127.0.0.1:%d", server.Port),
			Timeout: 5, // 5 second timeout
		},
		"slow-service": {
			Target:  fmt.Sprintf("127.0.0.1:%d", server.Port),
			Timeout: 30, // 30 second timeout
		},
	}

	manager := NewClientManager(configs, log)
	require.NotNil(t, manager)

	// Test that all clients can successfully connect (since the server is running properly)
	for serviceName := range configs {
		conn, err := manager.GetConn(serviceName)
		assert.NoError(t, err, "服务 %s 应该连接成功", serviceName)
		assert.NotNil(t, conn, "服务 %s 连接不应该为空", serviceName)

		// Verify connection availability
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		healthClient := grpc_health_v1.NewHealthClient(conn)
		resp, err := healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
		cancel()

		assert.NoError(t, err, "服务 %s 健康检查应该成功", serviceName)
		assert.Equal(t, grpc_health_v1.HealthCheckResponse_SERVING, resp.Status)
	}

	// Verify that the configuration is correctly read
	cfg1 := configs["fast-service"]
	assert.Equal(t, 1, cfg1.GetTimeout())

	cfg2 := configs["normal-service"]
	assert.Equal(t, 5, cfg2.GetTimeout())

	cfg3 := configs["slow-service"]
	assert.Equal(t, 30, cfg3.GetTimeout())
}

// TestClientManager_Timeout_RealTimeoutScenario Test real timeout scenario
// Use a service that delays responses to simulate a timeout
func TestClientManager_Timeout_RealTimeoutScenario(t *testing.T) {
	log := logger.GetLogger("grpc_test")

	// Start test server
	server := startTestGRPCServer(t)
	defer server.Stop(context.Background())

	// Configure 2-second timeout
	configs := map[string]ClientConfig{
		"test-service": {
			Target:  fmt.Sprintf("127.0.0.1:%d", server.Port),
			Timeout: 2, // 2 second timeout
		},
	}

	manager := NewClientManager(configs, log)
	require.NotNil(t, manager)

	// Get connection (should succeed)
	conn, err := manager.GetConn("test-service")
	require.NoError(t, err)
	require.NotNil(t, conn)

	// Create health check client
	healthClient := grpc_health_v1.NewHealthClient(conn)

	// Test 1: Normal call (should succeed)
	ctx1, cancel1 := context.WithTimeout(context.Background(), 1*time.Second)
	resp1, err1 := healthClient.Check(ctx1, &grpc_health_v1.HealthCheckRequest{})
	cancel1()

	assert.NoError(t, err1, "正常调用应该成功")
	assert.NotNil(t, resp1)

	// Test 2: Timeout call (simulating a timeout scenario using a 100ms timeout)
	ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Millisecond)
	start := time.Now()
	_, err2 := healthClient.Check(ctx2, &grpc_health_v1.HealthCheckRequest{
		Service: "delay-service", // Assume service will be delayed
	})
	elapsed := time.Since(start)
	cancel2()

	// Validate timeout behavior
	if err2 != nil {
		st, ok := status.FromError(err2)
		if ok {
			// Should be a timeout or cancellation error
			assert.True(t,
				st.Code() == codes.DeadlineExceeded || st.Code() == codes.Canceled,
				"应该是超时或取消错误，实际: %v", st.Code())
		}
	}
	assert.Less(t, elapsed, 200*time.Millisecond, "超时应该很快返回")

	t.Logf("超时测试完成，实际耗时: %v", elapsed)
}
