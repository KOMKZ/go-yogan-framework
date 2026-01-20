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
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// TestNewServer test create Server
func TestNewServer(t *testing.T) {
	log := logger.GetLogger("grpc_test")

	tests := []struct {
		name   string
		config ServerConfig
	}{
		{
			name: "默认配置",
			config: ServerConfig{
				Enabled:       true,
				Port:          0, // Automatically assign port
				MaxRecvSize:   4,
				MaxSendSize:   4,
				EnableReflect: true,
			},
		},
		{
			name: "禁用反射",
			config: ServerConfig{
				Enabled:       true,
				Port:          0,
				MaxRecvSize:   10,
				MaxSendSize:   10,
				EnableReflect: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewServer(tt.config, log)
			assert.NotNil(t, server)
			assert.Equal(t, tt.config.Port, server.Port)
			assert.NotNil(t, server.GetGRPCServer())
		})
	}
}

// TestServer_StartStop test server start and stop
func TestServer_StartStop(t *testing.T) {
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

	// Start server
	err := server.Start(context.Background())
	assert.NoError(t, err)
	assert.NotZero(t, server.Port, "端口应该被自动分配")

	// wait for the server to fully start up
	time.Sleep(100 * time.Millisecond)

	// Test connection (using health check)
	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()

	conn, err := grpc.DialContext(
		ctx2,
		fmt.Sprintf("127.0.0.1:%d", server.Port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err == nil {
		conn.Close()
	}

	// Shut down server
	server.Stop(context.Background())
}

// TestServer_StartStop_SpecificPort test start with specific port
func TestServer_StartStop_SpecificPort(t *testing.T) {
	log := logger.GetLogger("grpc_test")

	config := ServerConfig{
		Enabled:       true,
		Port:          50051, // specify port
		MaxRecvSize:   4,
		MaxSendSize:   4,
		EnableReflect: false,
	}

	server := NewServer(config, log)
	require.NotNil(t, server)

	// Start server
	err := server.Start(context.Background())
	if err != nil {
		// Port may be in use, skip test
		t.Skipf("端口 %d 被占用: %v", config.Port, err)
	}
	defer server.Stop(context.Background())

	assert.Equal(t, 50051, server.Port)

	// wait for the server to fully start up
	time.Sleep(100 * time.Millisecond)
}

// TestServer_GetGRPCServer test to retrieve the original gRPC server
func TestServer_GetGRPCServer(t *testing.T) {
	log := logger.GetLogger("grpc_test")

	config := ServerConfig{
		Enabled:       true,
		Port:          0,
		MaxRecvSize:   4,
		MaxSendSize:   4,
		EnableReflect: true,
	}

	server := NewServer(config, log)
	require.NotNil(t, server)

	grpcServer := server.GetGRPCServer()
	assert.NotNil(t, grpcServer)
	assert.IsType(t, &grpc.Server{}, grpcServer)
}

// TestServer_MultipleStartStop test multiple start and stop
func TestServer_MultipleStartStop(t *testing.T) {
	log := logger.GetLogger("grpc_test")

	config := ServerConfig{
		Enabled:       true,
		Port:          0,
		MaxRecvSize:   4,
		MaxSendSize:   4,
		EnableReflect: false,
	}

	for i := 0; i < 3; i++ {
		server := NewServer(config, log)
		require.NotNil(t, server)

		err := server.Start(context.Background())
		assert.NoError(t, err)

		time.Sleep(50 * time.Millisecond)

		server.Stop(context.Background())
	}
}

// TestServer_RegisterService test service registration
func TestServer_RegisterService(t *testing.T) {
	log := logger.GetLogger("grpc_test")

	config := ServerConfig{
		Enabled:       true,
		Port:          0,
		MaxRecvSize:   4,
		MaxSendSize:   4,
		EnableReflect: true,
	}

	server := NewServer(config, log)
	require.NotNil(t, server)

	// Register health check service (gRPC built-in)
	grpc_health_v1.RegisterHealthServer(server.GetGRPCServer(), &mockHealthServer{})

	err := server.Start(context.Background())
	assert.NoError(t, err)
	defer server.Stop(context.Background())

	time.Sleep(100 * time.Millisecond)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(
		ctx,
		fmt.Sprintf("127.0.0.1:%d", server.Port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err == nil {
		defer conn.Close()
		// Can call health check
		client := grpc_health_v1.NewHealthClient(conn)
		_, err := client.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
		assert.NoError(t, err)
	}
}

// mockHealthServer simulate health check service
type mockHealthServer struct {
	grpc_health_v1.UnimplementedHealthServer
}

func (s *mockHealthServer) Check(ctx context.Context, req *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	return &grpc_health_v1.HealthCheckResponse{
		Status: grpc_health_v1.HealthCheckResponse_SERVING,
	}, nil
}

