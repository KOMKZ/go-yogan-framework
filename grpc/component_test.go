package grpc

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/KOMKZ/go-yogan-framework/component"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// mockConfigLoader 模拟配置加载器
type mockConfigLoader struct {
	config Config
}

func (m *mockConfigLoader) Get(key string) interface{} {
	if key == "grpc" {
		return m.config
	}
	return nil
}

func (m *mockConfigLoader) Unmarshal(key string, target interface{}) error {
	if key == "grpc" {
		*(target.(*Config)) = m.config
		return nil
	}
	return fmt.Errorf("key not found: %s", key)
}

func (m *mockConfigLoader) GetString(key string) string {
	return ""
}

func (m *mockConfigLoader) GetInt(key string) int {
	return 0
}

func (m *mockConfigLoader) GetBool(key string) bool {
	return false
}

func (m *mockConfigLoader) IsSet(key string) bool {
	return key == "grpc"
}

// TestComponent_Name 测试组件名称
func TestComponent_Name(t *testing.T) {
	comp := NewComponent()
	assert.Equal(t, component.ComponentGRPC, comp.Name())
}

// TestComponent_DependsOn 测试组件依赖
func TestComponent_DependsOn(t *testing.T) {
	comp := NewComponent()
	deps := comp.DependsOn()

	assert.Len(t, deps, 2)
	assert.Contains(t, deps, component.ComponentConfig)
	assert.Contains(t, deps, component.ComponentLogger)
}

// TestComponent_Init_NoConfig 测试初始化（无配置）
func TestComponent_Init_NoConfig(t *testing.T) {
	comp := NewComponent()

	loader := &mockConfigLoader{
		config: Config{}, // 空配置
	}

	ctx := context.Background()
	err := comp.Init(ctx, loader)
	assert.NoError(t, err)
	assert.Nil(t, comp.GetServer())
	assert.Nil(t, comp.GetClientManager())
}

// TestComponent_Init_ServerOnly 测试初始化（仅服务端）
func TestComponent_Init_ServerOnly(t *testing.T) {
	comp := NewComponent()

	loader := &mockConfigLoader{
		config: Config{
			Server: ServerConfig{
				Enabled:       true,
				Port:          0,
				MaxRecvSize:   4,
				MaxSendSize:   4,
				EnableReflect: true,
			},
			Clients: nil,
		},
	}

	ctx := context.Background()
	err := comp.Init(ctx, loader)
	assert.NoError(t, err)
	assert.NotNil(t, comp.GetServer())
	assert.Nil(t, comp.GetClientManager())
}

// TestComponent_Init_ClientOnly 测试初始化（仅客户端）
func TestComponent_Init_ClientOnly(t *testing.T) {
	comp := NewComponent()

	loader := &mockConfigLoader{
		config: Config{
			Server: ServerConfig{
				Enabled: false,
			},
			Clients: map[string]ClientConfig{
				"service1": {
					Target:  "127.0.0.1:9001",
					Timeout: 5,
				},
			},
		},
	}

	ctx := context.Background()
	err := comp.Init(ctx, loader)
	assert.NoError(t, err)
	assert.Nil(t, comp.GetServer())
	assert.NotNil(t, comp.GetClientManager())
}

// TestComponent_Init_Both 测试初始化（服务端 + 客户端）
func TestComponent_Init_Both(t *testing.T) {
	comp := NewComponent()

	loader := &mockConfigLoader{
		config: Config{
			Server: ServerConfig{
				Enabled:       true,
				Port:          0,
				MaxRecvSize:   4,
				MaxSendSize:   4,
				EnableReflect: false,
			},
			Clients: map[string]ClientConfig{
				"service1": {
					Target:  "127.0.0.1:9001",
					Timeout: 5,
				},
			},
		},
	}

	ctx := context.Background()
	err := comp.Init(ctx, loader)
	assert.NoError(t, err)
	assert.NotNil(t, comp.GetServer())
	assert.NotNil(t, comp.GetClientManager())
}

// TestComponent_Lifecycle_ServerOnly 测试组件生命周期（仅服务端）
func TestComponent_Lifecycle_ServerOnly(t *testing.T) {
	comp := NewComponent()

	loader := &mockConfigLoader{
		config: Config{
			Server: ServerConfig{
				Enabled:       true,
				Port:          0,
				MaxRecvSize:   4,
				MaxSendSize:   4,
				EnableReflect: true,
			},
		},
	}

	ctx := context.Background()

	// 1. Init
	err := comp.Init(ctx, loader)
	require.NoError(t, err)
	require.NotNil(t, comp.GetServer())

	// 注册健康检查服务
	grpc_health_v1.RegisterHealthServer(comp.GetServer().GetGRPCServer(), &mockHealthServer{})

	// 2. Start
	err = comp.Start(ctx)
	require.NoError(t, err)
	assert.NotZero(t, comp.GetServer().Port)

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	// 3. Stop
	err = comp.Stop(ctx)
	assert.NoError(t, err)
}

// TestComponent_Lifecycle_ClientOnly 测试组件生命周期（仅客户端）
func TestComponent_Lifecycle_ClientOnly(t *testing.T) {
	// 先启动一个测试服务器
	logger, _ := zap.NewDevelopment()
	testServer := startTestGRPCServer(t)
	defer testServer.Stop(context.Background())

	comp := NewComponent()

	loader := &mockConfigLoader{
		config: Config{
			Server: ServerConfig{
				Enabled: false,
			},
			Clients: map[string]ClientConfig{
				"test-service": {
					Target:  fmt.Sprintf("127.0.0.1:%d", testServer.Port),
					Timeout: 5,
				},
			},
		},
	}

	ctx := context.Background()

	// 1. Init
	err := comp.Init(ctx, loader)
	require.NoError(t, err)
	require.NotNil(t, comp.GetClientManager())

	// 2. Start (客户端无需启动)
	err = comp.Start(ctx)
	require.NoError(t, err)

	// 3. 获取连接
	conn, err := comp.GetClientManager().GetConn("test-service")
	assert.NoError(t, err)
	assert.NotNil(t, conn)

	// 4. Stop
	err = comp.Stop(ctx)
	assert.NoError(t, err)

	// 验证连接已关闭
	ctx2, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	healthClient := grpc_health_v1.NewHealthClient(conn)
	_, err = healthClient.Check(ctx2, &grpc_health_v1.HealthCheckRequest{})
	assert.Error(t, err) // 连接已关闭，应该失败

	_ = logger // suppress unused warning
}

// TestComponent_Lifecycle_Full 测试完整组件生命周期（服务端 + 客户端）
func TestComponent_Lifecycle_Full(t *testing.T) {
	// 1. 创建第一个组件作为服务端
	serverComp := NewComponent()

	serverLoader := &mockConfigLoader{
		config: Config{
			Server: ServerConfig{
				Enabled:       true,
				Port:          0,
				MaxRecvSize:   4,
				MaxSendSize:   4,
				EnableReflect: true,
			},
		},
	}

	ctx := context.Background()

	err := serverComp.Init(ctx, serverLoader)
	require.NoError(t, err)

	grpc_health_v1.RegisterHealthServer(serverComp.GetServer().GetGRPCServer(), &mockHealthServer{})

	err = serverComp.Start(ctx)
	require.NoError(t, err)
	defer serverComp.Stop(ctx)

	time.Sleep(100 * time.Millisecond)

	// 2. 创建第二个组件作为客户端
	clientComp := NewComponent()

	clientLoader := &mockConfigLoader{
		config: Config{
			Server: ServerConfig{
				Enabled: false,
			},
			Clients: map[string]ClientConfig{
				"server": {
					Target:  fmt.Sprintf("127.0.0.1:%d", serverComp.GetServer().Port),
					Timeout: 5,
				},
			},
		},
	}

	err = clientComp.Init(ctx, clientLoader)
	require.NoError(t, err)

	err = clientComp.Start(ctx)
	require.NoError(t, err)
	defer clientComp.Stop(ctx)

	// 3. 测试客户端调用服务端
	conn, err := clientComp.GetClientManager().GetConn("server")
	require.NoError(t, err)
	require.NotNil(t, conn)

	healthClient := grpc_health_v1.NewHealthClient(conn)
	resp, err := healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	assert.NoError(t, err)
	assert.Equal(t, grpc_health_v1.HealthCheckResponse_SERVING, resp.Status)
}

// TestComponent_Stop_NilPointers 测试停止时的空指针处理
func TestComponent_Stop_NilPointers(t *testing.T) {
	comp := NewComponent()

	ctx := context.Background()

	// 未初始化直接停止（不应该 panic）
	err := comp.Stop(ctx)
	assert.NoError(t, err)
}

