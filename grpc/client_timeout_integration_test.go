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

// TestClientManager_RealTimeout_With10SecondDelay Test real scenario: server delay 10 seconds, client timeout 5 seconds
func TestClientManager_RealTimeout_With10SecondDelay(t *testing.T) {
	log := logger.GetLogger("grpc_test")

	// Start a slow test server (will delay responses)
	server := startSlowTestGRPCServer(t, 10*time.Second) // server delay of 10 seconds
	defer server.Stop(context.Background())

	// Configure client with 5 seconds timeout
	configs := map[string]ClientConfig{
		"slow-service": {
			Target:  fmt.Sprintf("127.0.0.1:%d", server.Port),
			Timeout: 5, // 5 second timeout
		},
	}

	manager := NewClientManager(configs, log)
	require.NotNil(t, manager)

	// The pre-connection should succeed (just establishing the TCP connection)
	manager.PreConnect(3 * time.Second)

	// Get connection
	conn, err := manager.GetConn("slow-service")
	require.NoError(t, err)
	require.NotNil(t, conn)

	// Create health check client
	healthClient := grpc_health_v1.NewHealthClient(conn)

	// The call should timeout within 5 seconds (although the server needs 10 seconds)
	ctx := context.Background()
	start := time.Now()
	_, err = healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	elapsed := time.Since(start)

	// Validate timeout
	t.Logf("调用耗时: %v", elapsed)
	assert.Error(t, err, "English: Should time out")

	// Verify timeout error
	st, ok := status.FromError(err)
	require.True(t, ok, "应该是 gRPC 错误")
	assert.Equal(t, codes.DeadlineExceeded, st.Code(), "应该是超时错误")

	// Validate timeout around 5 seconds (allowing ±1 second error)
	assert.Greater(t, elapsed, 4*time.Second, "至少等待4秒")
	assert.Less(t, elapsed, 7*time.Second, "不应超过7秒")

	t.Logf("✅ 超时测试通过：配置5秒超时，实际 %v 超时", elapsed)
}

// TestClientManager_DifferentTimeouts_RealDelay Test different timeout configurations
func TestClientManager_DifferentTimeouts_RealDelay(t *testing.T) {
	log := logger.GetLogger("grpc_test")

	// Start a slow test server (3 second delay)
	server := startSlowTestGRPCServer(t, 3*time.Second)
	defer server.Stop(context.Background())

	// Configure clients with different timeouts
	configs := map[string]ClientConfig{
		"fast-client": {
			Target:  fmt.Sprintf("127.0.0.1:%d", server.Port),
			Timeout: 1, // 1 second timeout (will time out)
		},
		"normal-client": {
			Target:  fmt.Sprintf("127.0.0.1:%d", server.Port),
			Timeout: 5, // 5 second timeout (success)
		},
	}

	manager := NewClientManager(configs, log)
	require.NotNil(t, manager)

	// Test 1: The client should fail with a 1-second timeout
	t.Run("1秒超时应该失败", func(t *testing.T) {
		conn, err := manager.GetConn("fast-client")
		require.NoError(t, err)

		healthClient := grpc_health_v1.NewHealthClient(conn)
		ctx := context.Background()

		start := time.Now()
		_, err = healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
		elapsed := time.Since(start)

		assert.Error(t, err, "English: Should time out")
		assert.Less(t, elapsed, 2*time.Second, "应该在2秒内超时")
		t.Logf("   1秒超时客户端耗时: %v", elapsed)
	})

	// Test 2: The client with a 5-second timeout should succeed
	t.Run("5秒超时应该成功", func(t *testing.T) {
		conn, err := manager.GetConn("normal-client")
		require.NoError(t, err)

		healthClient := grpc_health_v1.NewHealthClient(conn)
		ctx := context.Background()

		start := time.Now()
		resp, err := healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
		elapsed := time.Since(start)

		assert.NoError(t, err, "不应该超时")
		assert.NotNil(t, resp)
		assert.Greater(t, elapsed, 2*time.Second, "至少等待3秒左右")
		t.Logf("   5秒超时客户端耗时: %v (成功)", elapsed)
	})
}

// startSlowTestGRPCServer starts a slow test gRPC server
// delay: latency for each request
func startSlowTestGRPCServer(t *testing.T, delay time.Duration) *Server {
	log := logger.GetLogger("grpc_test")

	config := ServerConfig{
		Enabled:       true,
		Port:          0, // Automatically assign port
		MaxRecvSize:   4,
		MaxSendSize:   4,
		EnableReflect: false,
	}

	server := NewServer(config, log)

	// Services must be registered before Start
	grpc_health_v1.RegisterHealthServer(server.GetGRPCServer(), &slowHealthServer{delay: delay})

	// Start server
	go func() {
		if err := server.Start(context.Background()); err != nil {
			t.Logf("服务器启动失败: %v", err)
		}
	}()

	// wait for server to start
	time.Sleep(100 * time.Millisecond)

	return server
}

// slowHealthServer Slow health check service (simulated delay)
type slowHealthServer struct {
	grpc_health_v1.UnimplementedHealthServer
	delay time.Duration
}

func (s *slowHealthServer) Check(ctx context.Context, req *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	// simulate slow processing
	time.Sleep(s.delay)
	return &grpc_health_v1.HealthCheckResponse{
		Status: grpc_health_v1.HealthCheckResponse_SERVING,
	}, nil
}

