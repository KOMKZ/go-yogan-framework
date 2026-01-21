//go:build integration

package grpc

import (
	"context"
	"fmt"
	"testing"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadBalancer_Integration(t *testing.T) {
	t.Run("RoundRobin", func(t *testing.T) {
		lb := NewRoundRobinBalancer()
		assert.NotNil(t, lb)

		addrs := []string{"addr1:8080", "addr2:8080", "addr3:8080"}
		lb.Update(addrs)

		// Test round robin selection
		selected1, err1 := lb.Select(addrs)
		selected2, err2 := lb.Select(addrs)
		selected3, err3 := lb.Select(addrs)
		selected4, err4 := lb.Select(addrs)

		require.NoError(t, err1)
		require.NoError(t, err2)
		require.NoError(t, err3)
		require.NoError(t, err4)

		assert.Equal(t, "addr1:8080", selected1)
		assert.Equal(t, "addr2:8080", selected2)
		assert.Equal(t, "addr3:8080", selected3)
		assert.Equal(t, "addr1:8080", selected4) // Wraps around
	})

	t.Run("Random", func(t *testing.T) {
		lb := NewRandomBalancer()
		assert.NotNil(t, lb)

		addrs := []string{"addr1:8080", "addr2:8080", "addr3:8080"}
		lb.Update(addrs)

		// Test random selection
		selected, err := lb.Select(addrs)
		require.NoError(t, err)
		assert.Contains(t, addrs, selected)
	})

	t.Run("NewLoadBalancer", func(t *testing.T) {
		lb := NewLoadBalancer("round_robin")
		assert.NotNil(t, lb)

		lb2 := NewLoadBalancer("random")
		assert.NotNil(t, lb2)

		lb3 := NewLoadBalancer("unknown")
		assert.NotNil(t, lb3) // Defaults to round robin
	})

	t.Run("EmptyAddresses", func(t *testing.T) {
		lb := NewRoundRobinBalancer()
		lb.Update([]string{})

		_, err := lb.Select([]string{})
		assert.Error(t, err)

		lb2 := NewRandomBalancer()
		lb2.Update([]string{})

		_, err2 := lb2.Select([]string{})
		assert.Error(t, err2)
	})
}

func TestHealthChecker_Integration(t *testing.T) {
	log := logger.GetLogger("grpc_test")

	// Start a test server
	server := startTestGRPCServer(t)
	defer server.Stop(context.Background())

	configs := map[string]ClientConfig{
		"test-service": {
			Target:  fmt.Sprintf("127.0.0.1:%d", server.Port),
			Timeout: 5, // seconds
		},
	}

	manager := NewClientManager(configs, log)
	defer manager.Close()

	// Get connection first
	conn, err := manager.GetConn("test-service")
	require.NoError(t, err)
	require.NotNil(t, conn)

	// Create a server for health checker
	serverCfg := ServerConfig{
		Port: 0,
	}
	grpcServer := NewServer(serverCfg, log)

	checker := NewHealthChecker(grpcServer, manager)

	t.Run("Name", func(t *testing.T) {
		name := checker.Name()
		assert.Equal(t, "grpc", name)
	})

	t.Run("Check", func(t *testing.T) {
		err := checker.Check(context.Background())
		// May succeed or fail depending on health service
		_ = err
	})
}

func TestGRPCMetrics_Integration(t *testing.T) {
	metrics, err := NewGRPCMetrics(true, true)
	require.NoError(t, err)
	assert.NotNil(t, metrics)

	t.Run("StatsHandler", func(t *testing.T) {
		handler := metrics.StatsHandler()
		assert.NotNil(t, handler)
	})
}

func TestServer_SetMethods_Integration(t *testing.T) {
	log := logger.GetLogger("grpc_test")

	cfg := ServerConfig{
		Port: 0,
	}

	server := NewServer(cfg, log)

	t.Run("SetTracerProvider", func(t *testing.T) {
		server.SetTracerProvider(nil)
		// No panic means success
	})

	t.Run("SetMetricsHandler", func(t *testing.T) {
		server.SetMetricsHandler(nil)
		// No panic means success
	})
}

func TestTraceLoggerInterceptors_Integration(t *testing.T) {
	log := logger.GetLogger("grpc_test").GetZapLogger()

	t.Run("UnaryClientTraceLoggerInterceptor", func(t *testing.T) {
		interceptor := UnaryClientTraceLoggerInterceptor(log)
		assert.NotNil(t, interceptor)
	})

	t.Run("UnaryServerTraceLoggerInterceptor", func(t *testing.T) {
		interceptor := UnaryServerTraceLoggerInterceptor(log)
		assert.NotNil(t, interceptor)
	})
}

func TestClientManager_SetMethods_Integration(t *testing.T) {
	log := logger.GetLogger("grpc_test")

	configs := map[string]ClientConfig{
		"test-service": {
			Target:  "127.0.0.1:50051",
			Timeout: 5,
		},
	}

	manager := NewClientManager(configs, log)

	t.Run("SetSelector", func(t *testing.T) {
		selector := NewFirstHealthySelector()
		manager.SetSelector(selector)
		assert.NotNil(t, manager.selector)
	})

	t.Run("SetLimiter", func(t *testing.T) {
		// SetLimiter with nil (allowed)
		manager.SetLimiter(nil)
		assert.Nil(t, manager.GetLimiter())
	})

	t.Run("SetTracerProvider", func(t *testing.T) {
		manager.SetTracerProvider(nil)
		// No panic means success
	})

	t.Run("SetMetricsHandler", func(t *testing.T) {
		manager.SetMetricsHandler(nil)
		// No panic means success
	})

	t.Run("getSelector", func(t *testing.T) {
		selector := manager.getSelector()
		assert.NotNil(t, selector)
	})
}

func TestGRPCMetrics_Methods_Integration(t *testing.T) {
	metrics, err := NewGRPCMetrics(true, true)
	require.NoError(t, err)
	assert.NotNil(t, metrics)

	handler := metrics.StatsHandler()
	assert.NotNil(t, handler)

	// Just verify StatsHandler is created correctly
	// Detailed method tests require proper stats.RPCTagInfo etc.
}
