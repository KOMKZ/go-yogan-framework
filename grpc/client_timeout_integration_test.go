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

// TestClientManager_RealTimeout_With10SecondDelay 测试真实场景：服务端延迟10秒，客户端5秒超时
func TestClientManager_RealTimeout_With10SecondDelay(t *testing.T) {
	log := logger.GetLogger("grpc_test")

	// 启动一个慢速测试服务器（会延迟响应）
	server := startSlowTestGRPCServer(t, 10*time.Second) // 服务端延迟10秒
	defer server.Stop(context.Background())

	// 配置5秒超时的客户端
	configs := map[string]ClientConfig{
		"slow-service": {
			Target:  fmt.Sprintf("127.0.0.1:%d", server.Port),
			Timeout: 5, // 5秒超时
		},
	}

	manager := NewClientManager(configs, log)
	require.NotNil(t, manager)

	// 预连接应该成功（只是建立TCP连接）
	manager.PreConnect(3 * time.Second)

	// 获取连接
	conn, err := manager.GetConn("slow-service")
	require.NoError(t, err)
	require.NotNil(t, conn)

	// 创建健康检查客户端
	healthClient := grpc_health_v1.NewHealthClient(conn)

	// 调用应该在5秒内超时（虽然服务端需要10秒）
	ctx := context.Background()
	start := time.Now()
	_, err = healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	elapsed := time.Since(start)

	// 验证超时
	t.Logf("调用耗时: %v", elapsed)
	assert.Error(t, err, "应该超时")

	// 验证是超时错误
	st, ok := status.FromError(err)
	require.True(t, ok, "应该是 gRPC 错误")
	assert.Equal(t, codes.DeadlineExceeded, st.Code(), "应该是超时错误")

	// 验证在5秒左右超时（允许±1秒误差）
	assert.Greater(t, elapsed, 4*time.Second, "至少等待4秒")
	assert.Less(t, elapsed, 7*time.Second, "不应超过7秒")

	t.Logf("✅ 超时测试通过：配置5秒超时，实际 %v 超时", elapsed)
}

// TestClientManager_DifferentTimeouts_RealDelay 测试不同超时配置
func TestClientManager_DifferentTimeouts_RealDelay(t *testing.T) {
	log := logger.GetLogger("grpc_test")

	// 启动一个慢速测试服务器（延迟3秒）
	server := startSlowTestGRPCServer(t, 3*time.Second)
	defer server.Stop(context.Background())

	// 配置不同超时的客户端
	configs := map[string]ClientConfig{
		"fast-client": {
			Target:  fmt.Sprintf("127.0.0.1:%d", server.Port),
			Timeout: 1, // 1秒超时（会超时）
		},
		"normal-client": {
			Target:  fmt.Sprintf("127.0.0.1:%d", server.Port),
			Timeout: 5, // 5秒超时（能成功）
		},
	}

	manager := NewClientManager(configs, log)
	require.NotNil(t, manager)

	// 测试1: 1秒超时的客户端应该失败
	t.Run("1秒超时应该失败", func(t *testing.T) {
		conn, err := manager.GetConn("fast-client")
		require.NoError(t, err)

		healthClient := grpc_health_v1.NewHealthClient(conn)
		ctx := context.Background()

		start := time.Now()
		_, err = healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
		elapsed := time.Since(start)

		assert.Error(t, err, "应该超时")
		assert.Less(t, elapsed, 2*time.Second, "应该在2秒内超时")
		t.Logf("   1秒超时客户端耗时: %v", elapsed)
	})

	// 测试2: 5秒超时的客户端应该成功
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

// startSlowTestGRPCServer 启动一个慢速的测试gRPC服务器
// delay: 每个请求的延迟时间
func startSlowTestGRPCServer(t *testing.T, delay time.Duration) *Server {
	log := logger.GetLogger("grpc_test")

	config := ServerConfig{
		Enabled:       true,
		Port:          0, // 自动分配端口
		MaxRecvSize:   4,
		MaxSendSize:   4,
		EnableReflect: false,
	}

	server := NewServer(config, log)

	// 必须在 Start 之前注册服务
	grpc_health_v1.RegisterHealthServer(server.GetGRPCServer(), &slowHealthServer{delay: delay})

	// 启动服务器
	go func() {
		if err := server.Start(context.Background()); err != nil {
			t.Logf("服务器启动失败: %v", err)
		}
	}()

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	return server
}

// slowHealthServer 慢速健康检查服务（模拟延迟）
type slowHealthServer struct {
	grpc_health_v1.UnimplementedHealthServer
	delay time.Duration
}

func (s *slowHealthServer) Check(ctx context.Context, req *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	// 模拟慢速处理
	time.Sleep(s.delay)
	return &grpc_health_v1.HealthCheckResponse{
		Status: grpc_health_v1.HealthCheckResponse_SERVING,
	}, nil
}

