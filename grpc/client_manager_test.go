package grpc

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// TestNewClientManager 测试创建 ClientManager
func TestNewClientManager(t *testing.T) {
	log := logger.GetLogger("grpc_test")

	configs := map[string]ClientConfig{
		"service1": {
			Target:  "127.0.0.1:9001",
			Timeout: 5,
		},
		"service2": {
			Target:  "127.0.0.1:9002",
			Timeout: 3,
		},
	}

	manager := NewClientManager(configs, log)
	assert.NotNil(t, manager)
	assert.Equal(t, 2, len(manager.configs))
}

// TestClientManager_GetConn_NotConfigured 测试获取未配置的服务连接
func TestClientManager_GetConn_NotConfigured(t *testing.T) {
	log := logger.GetLogger("grpc_test")

	configs := map[string]ClientConfig{
		"service1": {
			Target:  "127.0.0.1:9001",
			Timeout: 5,
		},
	}

	manager := NewClientManager(configs, log)
	require.NotNil(t, manager)

	// 获取未配置的服务
	conn, err := manager.GetConn("not-exist-service")
	assert.Error(t, err)
	assert.Nil(t, conn)
	assert.Contains(t, err.Error(), "未配置服务")
}

// TestClientManager_GetConn_Success 测试成功获取连接
func TestClientManager_GetConn_Success(t *testing.T) {
	log := logger.GetLogger("grpc_test")

	// 启动一个测试服务器
	server := startTestGRPCServer(t)
	defer server.Stop(context.Background())

	configs := map[string]ClientConfig{
		"test-service": {
			Target:  fmt.Sprintf("127.0.0.1:%d", server.Port),
			Timeout: 5,
		},
	}

	manager := NewClientManager(configs, log)
	require.NotNil(t, manager)

	// 获取连接
	conn, err := manager.GetConn("test-service")
	assert.NoError(t, err)
	assert.NotNil(t, conn)

	// 测试连接可用
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	healthClient := grpc_health_v1.NewHealthClient(conn)
	resp, err := healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	assert.NoError(t, err)
	assert.Equal(t, grpc_health_v1.HealthCheckResponse_SERVING, resp.Status)
}

// TestClientManager_GetConn_Cached 测试连接缓存
func TestClientManager_GetConn_Cached(t *testing.T) {
	log := logger.GetLogger("grpc_test")

	// 启动一个测试服务器
	server := startTestGRPCServer(t)
	defer server.Stop(context.Background())

	configs := map[string]ClientConfig{
		"test-service": {
			Target:  fmt.Sprintf("127.0.0.1:%d", server.Port),
			Timeout: 5,
		},
	}

	manager := NewClientManager(configs, log)
	require.NotNil(t, manager)

	// 第一次获取连接
	conn1, err := manager.GetConn("test-service")
	assert.NoError(t, err)
	assert.NotNil(t, conn1)

	// 第二次获取连接（应该返回缓存的连接）
	conn2, err := manager.GetConn("test-service")
	assert.NoError(t, err)
	assert.NotNil(t, conn2)

	// 应该是同一个连接对象
	assert.Equal(t, conn1, conn2)
}

// TestClientManager_GetConn_ConnectionFailed 测试连接失败
func TestClientManager_GetConn_ConnectionFailed(t *testing.T) {
	log := logger.GetLogger("grpc_test")

	configs := map[string]ClientConfig{
		"invalid-service": {
			Target:  "127.0.0.1:99999", // 无效端口
			Timeout: 1,                 // 1 秒超时
		},
	}

	manager := NewClientManager(configs, log)
	require.NotNil(t, manager)

	// 获取连接（应该超时失败）
	conn, err := manager.GetConn("invalid-service")
	assert.Error(t, err)
	assert.Nil(t, conn)
	assert.Contains(t, err.Error(), "连接服务失败")
}

// TestClientManager_Close 测试关闭所有连接
func TestClientManager_Close(t *testing.T) {
	log := logger.GetLogger("grpc_test")

	// 启动两个测试服务器
	server1 := startTestGRPCServer(t)
	defer server1.Stop(context.Background())

	server2 := startTestGRPCServer(t)
	defer server2.Stop(context.Background())

	configs := map[string]ClientConfig{
		"service1": {
			Target:  fmt.Sprintf("127.0.0.1:%d", server1.Port),
			Timeout: 5,
		},
		"service2": {
			Target:  fmt.Sprintf("127.0.0.1:%d", server2.Port),
			Timeout: 5,
		},
	}

	manager := NewClientManager(configs, log)
	require.NotNil(t, manager)

	// 创建连接
	conn1, err := manager.GetConn("service1")
	assert.NoError(t, err)
	assert.NotNil(t, conn1)

	conn2, err := manager.GetConn("service2")
	assert.NoError(t, err)
	assert.NotNil(t, conn2)

	// 关闭所有连接
	manager.Close()

	// 验证连接已关闭（再次调用会失败）
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	healthClient := grpc_health_v1.NewHealthClient(conn1)
	_, err = healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	assert.Error(t, err) // 连接已关闭，应该失败
}

// TestClientManager_GetConn_Concurrent 测试并发获取连接
func TestClientManager_GetConn_Concurrent(t *testing.T) {
	log := logger.GetLogger("grpc_test")

	// 启动测试服务器
	server := startTestGRPCServer(t)
	defer server.Stop(context.Background())

	configs := map[string]ClientConfig{
		"test-service": {
			Target:  fmt.Sprintf("127.0.0.1:%d", server.Port),
			Timeout: 5,
		},
	}

	manager := NewClientManager(configs, log)
	require.NotNil(t, manager)

	// 并发获取连接
	const concurrency = 10
	conns := make([]*grpc.ClientConn, concurrency)
	errors := make([]error, concurrency)

	done := make(chan bool)
	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			conns[idx], errors[idx] = manager.GetConn("test-service")
			done <- true
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < concurrency; i++ {
		<-done
	}

	// 验证所有连接都成功且是同一个
	var firstConn *grpc.ClientConn
	for i := 0; i < concurrency; i++ {
		assert.NoError(t, errors[i])
		assert.NotNil(t, conns[i])

		if i == 0 {
			firstConn = conns[i]
		} else {
			assert.Equal(t, firstConn, conns[i], "所有连接应该是同一个实例")
		}
	}
}

// startTestGRPCServer 启动一个测试用 gRPC 服务器
func startTestGRPCServer(t *testing.T) *Server {
	log := logger.GetLogger("grpc_test")

	config := ServerConfig{
		Enabled:       true,
		Port:          0, // 自动分配端口
		MaxRecvSize:   4,
		MaxSendSize:   4,
		EnableReflect: true,
	}

	server := NewServer(config, log)
	require.NotNil(t, server)

	// 注册健康检查服务
	grpc_health_v1.RegisterHealthServer(server.GetGRPCServer(), &mockHealthServer{})

	err := server.Start(context.Background())
	require.NoError(t, err)

	// 等待服务器完全启动
	time.Sleep(100 * time.Millisecond)

	return server
}

