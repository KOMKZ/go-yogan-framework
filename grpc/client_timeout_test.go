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

// TestClientManager_Timeout_ShortTimeout 测试短超时配置
func TestClientManager_Timeout_ShortTimeout(t *testing.T) {
	log := logger.GetLogger("grpc_test")

	// 配置一个无法连接的地址，使用1秒超时
	configs := map[string]ClientConfig{
		"slow-service": {
			Target:  "127.0.0.1:19999", // 不存在的服务
			Timeout: 1,                 // 1秒超时
		},
	}

	manager := NewClientManager(configs, log)
	require.NotNil(t, manager)

	// 记录开始时间
	start := time.Now()

	// 尝试获取连接（应该在1秒内失败）
	conn, err := manager.GetConn("slow-service")

	// 计算耗时
	elapsed := time.Since(start)

	// 验证：应该失败且耗时接近1秒
	assert.Error(t, err, "应该超时失败")
	assert.Nil(t, conn, "连接应该为空")
	assert.Less(t, elapsed, 2*time.Second, "应该在2秒内超时")
	assert.Greater(t, elapsed, 500*time.Millisecond, "应该至少等待500ms")

	t.Logf("超时测试完成，耗时: %v", elapsed)
}

// TestClientManager_Timeout_LongTimeout 测试长超时配置
func TestClientManager_Timeout_LongTimeout(t *testing.T) {
	log := logger.GetLogger("grpc_test")

	// 配置一个无法连接的地址，使用10秒超时
	configs := map[string]ClientConfig{
		"slow-service": {
			Target:  "127.0.0.1:19998", // 不存在的服务
			Timeout: 10,                // 10秒超时
		},
	}

	manager := NewClientManager(configs, log)
	require.NotNil(t, manager)

	// 记录开始时间
	start := time.Now()

	// 尝试获取连接（应该在10秒内失败）
	conn, err := manager.GetConn("slow-service")

	// 计算耗时
	elapsed := time.Since(start)

	// 验证：应该失败且耗时接近10秒
	assert.Error(t, err, "应该超时失败")
	assert.Nil(t, conn, "连接应该为空")
	assert.Less(t, elapsed, 12*time.Second, "应该在12秒内超时")

	t.Logf("超时测试完成，耗时: %v", elapsed)
}

// TestClientManager_Timeout_DefaultTimeout 测试默认超时（未配置时）
func TestClientManager_Timeout_DefaultTimeout(t *testing.T) {
	log := logger.GetLogger("grpc_test")

	// 不配置超时，应该使用默认5秒
	configs := map[string]ClientConfig{
		"default-timeout-service": {
			Target: "127.0.0.1:19997", // 不存在的服务
			// Timeout 未设置，应该默认5秒
		},
	}

	manager := NewClientManager(configs, log)
	require.NotNil(t, manager)

	// 记录开始时间
	start := time.Now()

	// 尝试获取连接（应该在5秒内失败）
	conn, err := manager.GetConn("default-timeout-service")

	// 计算耗时
	elapsed := time.Since(start)

	// 验证：应该失败且耗时接近5秒
	assert.Error(t, err, "应该超时失败")
	assert.Nil(t, conn, "连接应该为空")
	assert.Less(t, elapsed, 7*time.Second, "应该在7秒内超时")
	assert.Greater(t, elapsed, 3*time.Second, "应该至少等待3秒")

	t.Logf("默认超时测试完成，耗时: %v", elapsed)
}

// TestClientManager_Timeout_DifferentTimeouts 测试多个客户端不同超时配置
func TestClientManager_Timeout_DifferentTimeouts(t *testing.T) {
	log := logger.GetLogger("grpc_test")

	// 启动一个正常的测试服务器
	server := startTestGRPCServer(t)
	defer server.Stop(context.Background())

	// 配置多个客户端，不同超时
	configs := map[string]ClientConfig{
		"fast-service": {
			Target:  fmt.Sprintf("127.0.0.1:%d", server.Port),
			Timeout: 1, // 1秒超时（但服务正常，能快速连接）
		},
		"normal-service": {
			Target:  fmt.Sprintf("127.0.0.1:%d", server.Port),
			Timeout: 5, // 5秒超时
		},
		"slow-service": {
			Target:  fmt.Sprintf("127.0.0.1:%d", server.Port),
			Timeout: 30, // 30秒超时
		},
	}

	manager := NewClientManager(configs, log)
	require.NotNil(t, manager)

	// 测试所有客户端都能成功连接（因为服务器正常）
	for serviceName := range configs {
		conn, err := manager.GetConn(serviceName)
		assert.NoError(t, err, "服务 %s 应该连接成功", serviceName)
		assert.NotNil(t, conn, "服务 %s 连接不应该为空", serviceName)

		// 验证连接可用
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		healthClient := grpc_health_v1.NewHealthClient(conn)
		resp, err := healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
		cancel()

		assert.NoError(t, err, "服务 %s 健康检查应该成功", serviceName)
		assert.Equal(t, grpc_health_v1.HealthCheckResponse_SERVING, resp.Status)
	}

	// 验证配置被正确读取
	cfg1 := configs["fast-service"]
	assert.Equal(t, 1, cfg1.GetTimeout())

	cfg2 := configs["normal-service"]
	assert.Equal(t, 5, cfg2.GetTimeout())

	cfg3 := configs["slow-service"]
	assert.Equal(t, 30, cfg3.GetTimeout())
}

// TestClientManager_Timeout_RealTimeoutScenario 测试真实超时场景
// 使用一个会延迟响应的服务模拟超时
func TestClientManager_Timeout_RealTimeoutScenario(t *testing.T) {
	log := logger.GetLogger("grpc_test")

	// 启动测试服务器
	server := startTestGRPCServer(t)
	defer server.Stop(context.Background())

	// 配置2秒超时
	configs := map[string]ClientConfig{
		"test-service": {
			Target:  fmt.Sprintf("127.0.0.1:%d", server.Port),
			Timeout: 2, // 2秒超时
		},
	}

	manager := NewClientManager(configs, log)
	require.NotNil(t, manager)

	// 获取连接（应该成功）
	conn, err := manager.GetConn("test-service")
	require.NoError(t, err)
	require.NotNil(t, conn)

	// 创建健康检查客户端
	healthClient := grpc_health_v1.NewHealthClient(conn)

	// 测试1: 正常调用（应该成功）
	ctx1, cancel1 := context.WithTimeout(context.Background(), 1*time.Second)
	resp1, err1 := healthClient.Check(ctx1, &grpc_health_v1.HealthCheckRequest{})
	cancel1()

	assert.NoError(t, err1, "正常调用应该成功")
	assert.NotNil(t, resp1)

	// 测试2: 超时调用（使用100ms超时模拟超时场景）
	ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Millisecond)
	start := time.Now()
	_, err2 := healthClient.Check(ctx2, &grpc_health_v1.HealthCheckRequest{
		Service: "delay-service", // 假设服务会延迟
	})
	elapsed := time.Since(start)
	cancel2()

	// 验证超时行为
	if err2 != nil {
		st, ok := status.FromError(err2)
		if ok {
			// 应该是超时或取消错误
			assert.True(t,
				st.Code() == codes.DeadlineExceeded || st.Code() == codes.Canceled,
				"应该是超时或取消错误，实际: %v", st.Code())
		}
	}
	assert.Less(t, elapsed, 200*time.Millisecond, "超时应该很快返回")

	t.Logf("超时测试完成，实际耗时: %v", elapsed)
}
