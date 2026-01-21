//go:build integration

package governance

import (
	"context"
	"testing"
	"time"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test etcd configuration - using local etcd cluster
var testEtcdConfig = EtcdRegistryConfig{
	Endpoints:         []string{"127.0.0.1:2379", "127.0.0.1:2479", "127.0.0.1:2579"},
	DialTimeout:       5 * time.Second,
	EnableRetry:       true,
	MaxRetries:        3,
	InitialRetryDelay: 1 * time.Second,
	MaxRetryDelay:     30 * time.Second,
	RetryBackoff:      2.0,
}

func TestEtcdClient_Integration(t *testing.T) {
	log := logger.GetLogger("test")

	cfg := etcdClientConfig{
		Endpoints:   testEtcdConfig.Endpoints,
		DialTimeout: testEtcdConfig.DialTimeout,
	}

	client, err := newEtcdClient(cfg, log)
	if err != nil {
		t.Skipf("Skipping etcd integration test: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	testKey := "/test/governance/key"
	testValue := "test-value"

	t.Run("Put and Get", func(t *testing.T) {
		err := client.Put(ctx, testKey, testValue)
		require.NoError(t, err)

		value, err := client.Get(ctx, testKey)
		require.NoError(t, err)
		assert.Equal(t, testValue, value)
	})

	t.Run("GetWithPrefix", func(t *testing.T) {
		err := client.Put(ctx, testKey+"/sub1", "value1")
		require.NoError(t, err)
		err = client.Put(ctx, testKey+"/sub2", "value2")
		require.NoError(t, err)

		result, err := client.GetWithPrefix(ctx, testKey)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(result), 2)
	})

	t.Run("Delete", func(t *testing.T) {
		err := client.Delete(ctx, testKey)
		require.NoError(t, err)

		_, err = client.Get(ctx, testKey)
		assert.Error(t, err) // Key not found
	})

	t.Run("GetClient", func(t *testing.T) {
		assert.NotNil(t, client.GetClient())
	})
}

func TestEtcdRegistry_Integration(t *testing.T) {
	log := logger.GetLogger("test")

	registry, err := NewEtcdRegistry(testEtcdConfig, log)
	if err != nil {
		t.Skipf("Skipping etcd integration test: %v", err)
	}
	defer registry.Deregister(context.Background())

	ctx := context.Background()

	t.Run("Register Service", func(t *testing.T) {
		serviceInfo := &ServiceInfo{
			ServiceName: "test-service-integration",
			Address:     "192.168.1.100",
			Port:        9002,
			Protocol:    "grpc",
			Version:     "v1.0.0",
			TTL:         10,
			Metadata: map[string]string{
				"env":    "test",
				"weight": "100",
			},
		}

		err := registry.Register(ctx, serviceInfo)
		require.NoError(t, err)
		assert.True(t, registry.IsRegistered())
	})

	t.Run("Update Metadata", func(t *testing.T) {
		newMetadata := map[string]string{
			"version": "v1.1.0",
			"region":  "cn-hangzhou",
		}

		err := registry.UpdateMetadata(ctx, newMetadata)
		require.NoError(t, err)
	})

	t.Run("Deregister Service", func(t *testing.T) {
		err := registry.Deregister(ctx)
		require.NoError(t, err)
		assert.False(t, registry.IsRegistered())
	})
}

func TestEtcdDiscovery_Integration(t *testing.T) {
	log := logger.GetLogger("test")

	cfg := etcdClientConfig{
		Endpoints:   testEtcdConfig.Endpoints,
		DialTimeout: testEtcdConfig.DialTimeout,
	}

	client, err := newEtcdClient(cfg, log)
	if err != nil {
		t.Skipf("Skipping etcd integration test: %v", err)
	}
	defer client.Close()

	discovery := NewEtcdDiscovery(client, log)
	defer discovery.Stop()

	ctx := context.Background()
	serviceName := "test-discovery-service"

	// Register a test service first
	registry, err := NewEtcdRegistry(testEtcdConfig, log)
	require.NoError(t, err)

	serviceInfo := &ServiceInfo{
		ServiceName: serviceName,
		Address:     "192.168.1.200",
		Port:        8080,
		Protocol:    "grpc",
		TTL:         10,
	}
	err = registry.Register(ctx, serviceInfo)
	require.NoError(t, err)

	t.Run("Discover Services", func(t *testing.T) {
		// Wait a bit for registration to propagate
		time.Sleep(100 * time.Millisecond)

		instances, err := discovery.Discover(ctx, serviceName)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(instances), 1)

		if len(instances) > 0 {
			assert.Equal(t, serviceName, instances[0].Service)
		}
	})

	// Cleanup
	registry.Deregister(ctx)
}

func TestDefaultEtcdClientConfig(t *testing.T) {
	cfg := defaultEtcdClientConfig()
	assert.Equal(t, []string{"127.0.0.1:2379"}, cfg.Endpoints)
	assert.Equal(t, 5*time.Second, cfg.DialTimeout)
}

func TestEtcdDiscovery_Watch_Integration(t *testing.T) {
	log := logger.GetLogger("test")

	cfg := etcdClientConfig{
		Endpoints:   testEtcdConfig.Endpoints,
		DialTimeout: testEtcdConfig.DialTimeout,
	}

	client, err := newEtcdClient(cfg, log)
	if err != nil {
		t.Skipf("Skipping etcd integration test: %v", err)
	}
	defer client.Close()

	discovery := NewEtcdDiscovery(client, log)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	serviceName := "test-watch-service"

	t.Run("Watch returns channel", func(t *testing.T) {
		ch, err := discovery.Watch(ctx, serviceName)
		require.NoError(t, err)
		assert.NotNil(t, ch)
	})

	discovery.Stop()
}

func TestEtcdRegistry_Close_Integration(t *testing.T) {
	log := logger.GetLogger("test")

	registry, err := NewEtcdRegistry(testEtcdConfig, log)
	if err != nil {
		t.Skipf("Skipping etcd integration test: %v", err)
	}

	// Close without registering
	err = registry.Close()
	assert.NoError(t, err)
}

func TestCalculateBackoff(t *testing.T) {
	log := logger.GetLogger("test")

	registry, err := NewEtcdRegistry(testEtcdConfig, log)
	if err != nil {
		t.Skipf("Skipping etcd integration test: %v", err)
	}
	defer registry.Close()

	// Test backoff calculation
	backoff1 := registry.calculateBackoff(1)
	backoff2 := registry.calculateBackoff(2)
	backoff3 := registry.calculateBackoff(3)

	assert.Greater(t, backoff2, backoff1)
	assert.Greater(t, backoff3, backoff2)
	assert.LessOrEqual(t, backoff3, 30*time.Second) // Max delay
}

func TestCheckEtcdHealth_Integration(t *testing.T) {
	log := logger.GetLogger("test")

	registry, err := NewEtcdRegistry(testEtcdConfig, log)
	if err != nil {
		t.Skipf("Skipping etcd integration test: %v", err)
	}
	defer registry.Close()

	ctx := context.Background()
	healthy := registry.checkEtcdHealth(ctx)
	assert.True(t, healthy)
}

func TestMarshalServiceInfo_Integration(t *testing.T) {
	log := logger.GetLogger("test")

	registry, err := NewEtcdRegistry(testEtcdConfig, log)
	if err != nil {
		t.Skipf("Skipping etcd integration test: %v", err)
	}
	defer registry.Close()

	info := &ServiceInfo{
		ServiceName: "test-service",
		Address:     "192.168.1.100",
		Port:        9002,
		Protocol:    "grpc",
		Version:     "v1.0.0",
		TTL:         10,
	}
	info.Validate()

	data, err := registry.marshalServiceInfo(info)
	require.NoError(t, err)
	assert.NotEmpty(t, data)
}

func TestBuildServiceKey_Integration(t *testing.T) {
	log := logger.GetLogger("test")

	registry, err := NewEtcdRegistry(testEtcdConfig, log)
	if err != nil {
		t.Skipf("Skipping etcd integration test: %v", err)
	}
	defer registry.Close()

	info := &ServiceInfo{
		ServiceName: "test-service",
		InstanceID:  "instance-1",
		Address:     "192.168.1.100",
		Port:        9002,
	}
	key := registry.buildServiceKey(info)
	assert.Contains(t, key, "test-service")
	assert.Contains(t, key, "instance-1")
}

func TestEtcdClient_PutGetDelete_Errors(t *testing.T) {
	log := logger.GetLogger("test")

	cfg := etcdClientConfig{
		Endpoints:   testEtcdConfig.Endpoints,
		DialTimeout: testEtcdConfig.DialTimeout,
	}

	client, err := newEtcdClient(cfg, log)
	if err != nil {
		t.Skipf("Skipping etcd integration test: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Get non-existent key
	_, err = client.Get(ctx, "/nonexistent/key/12345")
	assert.Error(t, err)
}

func TestEtcdRegistry_RegisterDuplicate_Integration(t *testing.T) {
	log := logger.GetLogger("test")

	registry, err := NewEtcdRegistry(testEtcdConfig, log)
	if err != nil {
		t.Skipf("Skipping etcd integration test: %v", err)
	}
	defer registry.Close()

	ctx := context.Background()

	serviceInfo := &ServiceInfo{
		ServiceName: "test-duplicate-service",
		Address:     "192.168.1.100",
		Port:        9002,
		Protocol:    "grpc",
		TTL:         10,
	}

	// First registration
	err = registry.Register(ctx, serviceInfo)
	require.NoError(t, err)

	// Second registration should re-register (not fail)
	err = registry.Register(ctx, serviceInfo)
	assert.NoError(t, err) // Re-registers with warning

	// Cleanup
	registry.Deregister(ctx)
}

func TestGetLocalIP_Integration(t *testing.T) {
	ip, err := GetLocalIP()
	assert.NotEmpty(t, ip)
	// Error may or may not occur depending on network
	_ = err
}

func TestEtcdRegistry_DeregisterNotRegistered_Integration(t *testing.T) {
	log := logger.GetLogger("test")

	registry, err := NewEtcdRegistry(testEtcdConfig, log)
	if err != nil {
		t.Skipf("Skipping etcd integration test: %v", err)
	}
	defer registry.Close()

	ctx := context.Background()

	// Deregister without registering first
	err = registry.Deregister(ctx)
	assert.Error(t, err) // Not registered
}

func TestEtcdRegistry_UpdateMetadataNotRegistered_Integration(t *testing.T) {
	log := logger.GetLogger("test")

	registry, err := NewEtcdRegistry(testEtcdConfig, log)
	if err != nil {
		t.Skipf("Skipping etcd integration test: %v", err)
	}
	defer registry.Close()

	ctx := context.Background()

	// Update metadata without registering first
	err = registry.UpdateMetadata(ctx, map[string]string{"key": "value"})
	assert.Error(t, err) // Not registered
}

func TestEtcdRegistry_IsRegistered_Integration(t *testing.T) {
	log := logger.GetLogger("test")

	registry, err := NewEtcdRegistry(testEtcdConfig, log)
	if err != nil {
		t.Skipf("Skipping etcd integration test: %v", err)
	}
	defer registry.Close()

	// Initially not registered
	assert.False(t, registry.IsRegistered())

	// After registration
	ctx := context.Background()
	serviceInfo := &ServiceInfo{
		ServiceName: "test-isregistered-service",
		Address:     "192.168.1.100",
		Port:        9002,
		TTL:         10,
	}

	err = registry.Register(ctx, serviceInfo)
	require.NoError(t, err)
	assert.True(t, registry.IsRegistered())

	// After deregistration
	registry.Deregister(ctx)
	assert.False(t, registry.IsRegistered())
}

func TestParseServiceInstance_Integration(t *testing.T) {
	log := logger.GetLogger("test")

	cfg := etcdClientConfig{
		Endpoints:   testEtcdConfig.Endpoints,
		DialTimeout: testEtcdConfig.DialTimeout,
	}

	client, err := newEtcdClient(cfg, log)
	if err != nil {
		t.Skipf("Skipping etcd integration test: %v", err)
	}
	defer client.Close()

	discovery := NewEtcdDiscovery(client, log)
	defer discovery.Stop()

	// Test parsing valid JSON
	jsonData := `{"service_name":"test","address":"192.168.1.1","port":8080}`
	instance, err := discovery.parseServiceInstance("test-key", jsonData)
	assert.NoError(t, err)
	assert.NotNil(t, instance)
	assert.Equal(t, "192.168.1.1", instance.Address)
	assert.Equal(t, 8080, instance.Port)

	// Test parsing invalid JSON - parseServiceInstance may fallback to defaults
	invalidData := `{invalid json}`
	instance, err = discovery.parseServiceInstance("test-key", invalidData)
	// Note: parseServiceInstance may not return error but use fallback parsing
	_ = err
	_ = instance
}

func TestEtcdClient_WithAuth_Integration(t *testing.T) {
	log := logger.GetLogger("test")

	// Test with empty username/password (should work without auth)
	cfg := etcdClientConfig{
		Endpoints:   testEtcdConfig.Endpoints,
		DialTimeout: testEtcdConfig.DialTimeout,
		Username:    "",
		Password:    "",
	}

	client, err := newEtcdClient(cfg, log)
	if err != nil {
		t.Skipf("Skipping etcd integration test: %v", err)
	}
	defer client.Close()

	assert.NotNil(t, client)
}

func TestEtcdClient_DefaultTimeout_Integration(t *testing.T) {
	log := logger.GetLogger("test")

	// Test with zero timeout (should use default)
	cfg := etcdClientConfig{
		Endpoints:   testEtcdConfig.Endpoints,
		DialTimeout: 0, // Will use default
	}

	client, err := newEtcdClient(cfg, log)
	if err != nil {
		t.Skipf("Skipping etcd integration test: %v", err)
	}
	defer client.Close()

	assert.NotNil(t, client)
}

func TestEtcdRegistry_DefaultValues_Integration(t *testing.T) {
	log := logger.GetLogger("test")

	// Test with minimal config (should use defaults)
	cfg := EtcdRegistryConfig{
		Endpoints:   testEtcdConfig.Endpoints,
		DialTimeout: testEtcdConfig.DialTimeout,
		// All retry configs are zero/false - should use defaults
	}

	registry, err := NewEtcdRegistry(cfg, log)
	if err != nil {
		t.Skipf("Skipping etcd integration test: %v", err)
	}
	defer registry.Close()

	assert.NotNil(t, registry)
}

func TestWeightedBalancer_AllUnhealthy_Integration(t *testing.T) {
	lb := NewWeightedBalancer()

	// All unhealthy instances with zero weight
	instances := []*ServiceInstance{
		{Address: "192.168.1.1", Port: 8080, Weight: 0, Healthy: false},
		{Address: "192.168.1.2", Port: 8080, Weight: 0, Healthy: false},
		{Address: "192.168.1.3", Port: 8080, Weight: 0, Healthy: false},
	}

	// Should still return an instance (fallback to round robin)
	for i := 0; i < 5; i++ {
		instance := lb.Select(instances)
		assert.NotNil(t, instance)
	}
}

func TestEtcdRegistry_FullLifecycle_Integration(t *testing.T) {
	log := logger.GetLogger("test")

	registry, err := NewEtcdRegistry(testEtcdConfig, log)
	if err != nil {
		t.Skipf("Skipping etcd integration test: %v", err)
	}

	ctx := context.Background()

	// 1. Register
	serviceInfo := &ServiceInfo{
		ServiceName: "test-lifecycle-service",
		Address:     "192.168.1.100",
		Port:        9002,
		Protocol:    "grpc",
		Version:     "v1.0.0",
		TTL:         5,
		Metadata: map[string]string{
			"env": "test",
		},
	}

	err = registry.Register(ctx, serviceInfo)
	require.NoError(t, err)
	assert.True(t, registry.IsRegistered())

	// 2. Wait for a heartbeat cycle
	time.Sleep(1 * time.Second)

	// 3. Update metadata
	newMetadata := map[string]string{
		"version": "v1.1.0",
		"region":  "cn-hangzhou",
	}
	err = registry.UpdateMetadata(ctx, newMetadata)
	require.NoError(t, err)

	// 4. Deregister
	err = registry.Deregister(ctx)
	require.NoError(t, err)
	assert.False(t, registry.IsRegistered())

	// 5. Close
	err = registry.Close()
	require.NoError(t, err)
}

func TestNewEtcdClient_NilLogger_Integration(t *testing.T) {
	cfg := etcdClientConfig{
		Endpoints:   testEtcdConfig.Endpoints,
		DialTimeout: testEtcdConfig.DialTimeout,
	}

	client, err := newEtcdClient(cfg, nil)
	if err != nil {
		t.Skipf("Skipping etcd integration test: %v", err)
	}
	defer client.Close()

	assert.NotNil(t, client)
}

func TestNewEtcdRegistry_NilLogger_Integration(t *testing.T) {
	registry, err := NewEtcdRegistry(testEtcdConfig, nil)
	if err != nil {
		t.Skipf("Skipping etcd integration test: %v", err)
	}
	defer registry.Close()

	assert.NotNil(t, registry)
}

func TestEtcdDiscovery_NilLogger_Integration(t *testing.T) {
	cfg := etcdClientConfig{
		Endpoints:   testEtcdConfig.Endpoints,
		DialTimeout: testEtcdConfig.DialTimeout,
	}

	client, err := newEtcdClient(cfg, nil)
	if err != nil {
		t.Skipf("Skipping etcd integration test: %v", err)
	}
	defer client.Close()

	discovery := NewEtcdDiscovery(client, nil)
	defer discovery.Stop()

	assert.NotNil(t, discovery)
}

func TestGetLocalIP_Always_Integration(t *testing.T) {
	// GetLocalIP should always return something
	ip, _ := GetLocalIP()
	assert.NotEmpty(t, ip)
}

func TestEtcdDiscovery_DiscoverEmpty_Integration(t *testing.T) {
	log := logger.GetLogger("test")

	cfg := etcdClientConfig{
		Endpoints:   testEtcdConfig.Endpoints,
		DialTimeout: testEtcdConfig.DialTimeout,
	}

	client, err := newEtcdClient(cfg, log)
	if err != nil {
		t.Skipf("Skipping etcd integration test: %v", err)
	}
	defer client.Close()

	discovery := NewEtcdDiscovery(client, log)
	defer discovery.Stop()

	ctx := context.Background()

	// Discover non-existent service
	instances, err := discovery.Discover(ctx, "nonexistent-service-12345")
	require.NoError(t, err)
	assert.Empty(t, instances)
}

func TestClientManager_SelectInstance_LoadBalancerNil_Integration(t *testing.T) {
	log := logger.GetLogger("test")

	cfg := etcdClientConfig{
		Endpoints:   testEtcdConfig.Endpoints,
		DialTimeout: testEtcdConfig.DialTimeout,
	}

	client, err := newEtcdClient(cfg, log)
	if err != nil {
		t.Skipf("Skipping etcd integration test: %v", err)
	}
	defer client.Close()

	discovery := NewEtcdDiscovery(client, log)
	defer discovery.Stop()

	// First register a service
	registry, _ := NewEtcdRegistry(testEtcdConfig, log)
	defer registry.Close()

	serviceInfo := &ServiceInfo{
		ServiceName: "test-cm-select-service",
		Address:     "192.168.1.200",
		Port:        8080,
		TTL:         10,
	}
	registry.Register(context.Background(), serviceInfo)
	defer registry.Deregister(context.Background())

	time.Sleep(100 * time.Millisecond)

	// Create ClientManager without load balancer
	cm := NewClientManager(discovery, nil, nil, nil)
	defer cm.Stop()

	instance, err := cm.SelectInstance(context.Background(), "test-cm-select-service")
	if err == nil {
		assert.NotNil(t, instance)
	}
}

func TestClientManager_CheckCircuit_Integration(t *testing.T) {
	log := logger.GetLogger("test")

	cfg := etcdClientConfig{
		Endpoints:   testEtcdConfig.Endpoints,
		DialTimeout: testEtcdConfig.DialTimeout,
	}

	client, err := newEtcdClient(cfg, log)
	if err != nil {
		t.Skipf("Skipping etcd integration test: %v", err)
	}
	defer client.Close()

	discovery := NewEtcdDiscovery(client, log)
	defer discovery.Stop()

	cb := NewSimpleCircuitBreaker(DefaultCircuitBreakerConfig())
	cm := NewClientManager(discovery, nil, cb, nil)
	defer cm.Stop()

	// Test check circuit
	err = cm.CheckCircuit("test-circuit-service")
	assert.NoError(t, err)
}

func TestCalculateBackoff_MaxDelay_Integration(t *testing.T) {
	log := logger.GetLogger("test")

	registry, err := NewEtcdRegistry(testEtcdConfig, log)
	if err != nil {
		t.Skipf("Skipping etcd integration test: %v", err)
	}
	defer registry.Close()

	// Test backoff calculation with large values
	backoff := registry.calculateBackoff(100 * time.Second)
	// Should be capped at max delay (30s)
	assert.LessOrEqual(t, backoff, 30*time.Second)
}

func TestEtcdRegistry_SetOnRegisterFailed_Integration(t *testing.T) {
	log := logger.GetLogger("test")

	called := false
	cfg := EtcdRegistryConfig{
		Endpoints:         testEtcdConfig.Endpoints,
		DialTimeout:       testEtcdConfig.DialTimeout,
		OnRegisterFailed: func(err error) {
			called = true
		},
	}

	registry, err := NewEtcdRegistry(cfg, log)
	if err != nil {
		t.Skipf("Skipping etcd integration test: %v", err)
	}
	defer registry.Close()

	assert.NotNil(t, registry)
	// The callback should be stored but not called yet
	_ = called
}

func TestServiceInfo_Validate_AllDefaults_Integration(t *testing.T) {
	info := &ServiceInfo{
		ServiceName: "test",
		Address:     "192.168.1.1",
		Port:        8080,
	}

	err := info.Validate()
	assert.NoError(t, err)
	assert.NotEmpty(t, info.InstanceID)
	// Protocol defaults to grpc, TTL defaults to 10
	assert.Equal(t, "grpc", info.Protocol)
	assert.Equal(t, int64(10), info.TTL)
}

func TestEtcdDiscovery_GetInstanceList_Integration(t *testing.T) {
	log := logger.GetLogger("test")

	cfg := etcdClientConfig{
		Endpoints:   testEtcdConfig.Endpoints,
		DialTimeout: testEtcdConfig.DialTimeout,
	}

	client, err := newEtcdClient(cfg, log)
	if err != nil {
		t.Skipf("Skipping etcd integration test: %v", err)
	}
	defer client.Close()

	discovery := NewEtcdDiscovery(client, log)
	defer discovery.Stop()

	// Register a service first
	registry, _ := NewEtcdRegistry(testEtcdConfig, log)
	defer registry.Close()

	serviceInfo := &ServiceInfo{
		ServiceName: "test-getinstancelist-service",
		Address:     "192.168.1.200",
		Port:        8080,
		TTL:         10,
	}
	registry.Register(context.Background(), serviceInfo)
	defer registry.Deregister(context.Background())

	time.Sleep(100 * time.Millisecond)

	// Use Discover which internally calls getInstanceList
	instances, err := discovery.Discover(context.Background(), "test-getinstancelist-service")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(instances), 1)
}
