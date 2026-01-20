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

// TestNewClientManager test creating ClientManager
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

// TestClientManager_GetConn_NotConfigured test getting a connection for an unconfigured service
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

	// Get unconfigured services
	conn, err := manager.GetConn("not-exist-service")
	assert.Error(t, err)
	assert.Nil(t, conn)
	assert.Contains(t, err.Error(), "Service not configured")
}

// TestClientManager_GetConn_Success Successfully obtained connection
func TestClientManager_GetConn_Success(t *testing.T) {
	log := logger.GetLogger("grpc_test")

	// Start a test server
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

	// Get connection
	conn, err := manager.GetConn("test-service")
	assert.NoError(t, err)
	assert.NotNil(t, conn)

	// Test connection availability
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	healthClient := grpc_health_v1.NewHealthClient(conn)
	resp, err := healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	assert.NoError(t, err)
	assert.Equal(t, grpc_health_v1.HealthCheckResponse_SERVING, resp.Status)
}

// TestClientManager_GetConn_Cached test connection cache
func TestClientManager_GetConn_Cached(t *testing.T) {
	log := logger.GetLogger("grpc_test")

	// Start a test server
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

	// First connection retrieval
	conn1, err := manager.GetConn("test-service")
	assert.NoError(t, err)
	assert.NotNil(t, conn1)

	// The second attempt to get a connection (should return a cached connection)
	conn2, err := manager.GetConn("test-service")
	assert.NoError(t, err)
	assert.NotNil(t, conn2)

	// It should be the same connection object
	assert.Equal(t, conn1, conn2)
}

// TestClientManager_GetConn_ConnectionFailed_Test connection failed
func TestClientManager_GetConn_ConnectionFailed(t *testing.T) {
	log := logger.GetLogger("grpc_test")

	configs := map[string]ClientConfig{
		"invalid-service": {
			Target:  "127.0.0.1:99999", // invalid port
			Timeout: 1,                 // 1 second timeout
		},
	}

	manager := NewClientManager(configs, log)
	require.NotNil(t, manager)

	// Get connection (should timeout and fail)
	conn, err := manager.GetConn("invalid-service")
	assert.Error(t, err)
	assert.Nil(t, conn)
	assert.Contains(t, err.Error(), "Connection to service failed")
}

// TestClientManager_Close_TestCloseAllConnections
func TestClientManager_Close(t *testing.T) {
	log := logger.GetLogger("grpc_test")

	// Start two test servers
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

	// Establish connection
	conn1, err := manager.GetConn("service1")
	assert.NoError(t, err)
	assert.NotNil(t, conn1)

	conn2, err := manager.GetConn("service2")
	assert.NoError(t, err)
	assert.NotNil(t, conn2)

	// Close all connections
	manager.Close()

	// Verify that the connection is closed (calling again will fail)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	healthClient := grpc_health_v1.NewHealthClient(conn1)
	_, err = healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	assert.Error(t, err) // 连接已关闭，应该失败
}

// TestClientManager_GetConn-Concurrent Test concurrent connection acquisition
func TestClientManager_GetConn_Concurrent(t *testing.T) {
	log := logger.GetLogger("grpc_test")

	// Start test server
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

	// concurrently obtain connection
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

	// wait for all goroutines to finish
	for i := 0; i < concurrency; i++ {
		<-done
	}

	// Verify that all connections are successful and belong to the same entity
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

// startTestGRPCServer Starts a test gRPC server
func startTestGRPCServer(t *testing.T) *Server {
	log := logger.GetLogger("grpc_test")

	config := ServerConfig{
		Enabled:       true,
		Port:          0, // Automatically assign port
		MaxRecvSize:   4,
		MaxSendSize:   4,
		EnableReflect: true,
	}

	server := NewServer(config, log)
	require.NotNil(t, server)

	// Register health check service
	grpc_health_v1.RegisterHealthServer(server.GetGRPCServer(), &mockHealthServer{})

	err := server.Start(context.Background())
	require.NoError(t, err)

	// wait for the server to fully start up
	time.Sleep(100 * time.Millisecond)

	return server
}

