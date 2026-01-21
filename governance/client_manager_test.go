package governance

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// MockDiscovery mock service discovery
type MockDiscovery struct {
	DiscoverFunc func(ctx context.Context, serviceName string) ([]*ServiceInstance, error)
	WatchFunc    func(ctx context.Context, serviceName string) (<-chan []*ServiceInstance, error)
	StopFunc     func()
}

func (m *MockDiscovery) Discover(ctx context.Context, serviceName string) ([]*ServiceInstance, error) {
	if m.DiscoverFunc != nil {
		return m.DiscoverFunc(ctx, serviceName)
	}
	return nil, nil
}

func (m *MockDiscovery) Watch(ctx context.Context, serviceName string) (<-chan []*ServiceInstance, error) {
	if m.WatchFunc != nil {
		return m.WatchFunc(ctx, serviceName)
	}
	return nil, nil
}

func (m *MockDiscovery) Stop() {
	if m.StopFunc != nil {
		m.StopFunc()
	}
}

func TestNewClientManager(t *testing.T) {
	t.Run("with nil logger", func(t *testing.T) {
		cm := NewClientManager(nil, nil, nil, nil)
		assert.NotNil(t, cm)
	})

	t.Run("with logger", func(t *testing.T) {
		logger := zap.NewNop()
		cm := NewClientManager(nil, nil, nil, logger)
		assert.NotNil(t, cm)
	})
}

func TestClientManager_Discover(t *testing.T) {
	t.Run("no discovery configured", func(t *testing.T) {
		cm := NewClientManager(nil, nil, nil, nil)
		_, err := cm.Discover(context.Background(), "test-service")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "service discovery not configured")
	})

	t.Run("discovery returns instances", func(t *testing.T) {
		discovery := &MockDiscovery{
			DiscoverFunc: func(ctx context.Context, serviceName string) ([]*ServiceInstance, error) {
				return []*ServiceInstance{
					{Address: "192.168.1.1", Port: 8080, Healthy: true},
				}, nil
			},
		}
		cm := NewClientManager(discovery, nil, nil, nil)
		instances, err := cm.Discover(context.Background(), "test-service")
		assert.NoError(t, err)
		assert.Len(t, instances, 1)
	})
}

func TestClientManager_SelectInstance(t *testing.T) {
	t.Run("discovery error", func(t *testing.T) {
		discovery := &MockDiscovery{
			DiscoverFunc: func(ctx context.Context, serviceName string) ([]*ServiceInstance, error) {
				return nil, errors.New("discovery failed")
			},
		}
		cm := NewClientManager(discovery, nil, nil, nil)
		_, err := cm.SelectInstance(context.Background(), "test-service")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "discover service failed")
	})

	t.Run("no instances available", func(t *testing.T) {
		discovery := &MockDiscovery{
			DiscoverFunc: func(ctx context.Context, serviceName string) ([]*ServiceInstance, error) {
				return []*ServiceInstance{}, nil
			},
		}
		cm := NewClientManager(discovery, nil, nil, nil)
		_, err := cm.SelectInstance(context.Background(), "test-service")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no available instances")
	})

	t.Run("with healthy instances and load balancer", func(t *testing.T) {
		discovery := &MockDiscovery{
			DiscoverFunc: func(ctx context.Context, serviceName string) ([]*ServiceInstance, error) {
				return []*ServiceInstance{
					{Address: "192.168.1.1", Port: 8080, Healthy: true},
					{Address: "192.168.1.2", Port: 8080, Healthy: true},
				}, nil
			},
		}
		lb := NewRoundRobinBalancer()
		cm := NewClientManager(discovery, lb, nil, nil)

		instance, err := cm.SelectInstance(context.Background(), "test-service")
		assert.NoError(t, err)
		assert.NotNil(t, instance)
	})

	t.Run("all instances unhealthy - fallback", func(t *testing.T) {
		discovery := &MockDiscovery{
			DiscoverFunc: func(ctx context.Context, serviceName string) ([]*ServiceInstance, error) {
				return []*ServiceInstance{
					{Address: "192.168.1.1", Port: 8080, Healthy: false},
				}, nil
			},
		}
		cm := NewClientManager(discovery, nil, nil, zap.NewNop())

		instance, err := cm.SelectInstance(context.Background(), "test-service")
		assert.NoError(t, err)
		assert.NotNil(t, instance)
	})

	t.Run("without load balancer", func(t *testing.T) {
		discovery := &MockDiscovery{
			DiscoverFunc: func(ctx context.Context, serviceName string) ([]*ServiceInstance, error) {
				return []*ServiceInstance{
					{Address: "192.168.1.1", Port: 8080, Healthy: true},
				}, nil
			},
		}
		cm := NewClientManager(discovery, nil, nil, nil)

		instance, err := cm.SelectInstance(context.Background(), "test-service")
		assert.NoError(t, err)
		assert.NotNil(t, instance)
	})
}

func TestClientManager_CircuitBreaker(t *testing.T) {
	cb := NewSimpleCircuitBreaker(DefaultCircuitBreakerConfig())
	cm := NewClientManager(nil, nil, cb, nil)

	t.Run("check circuit - closed", func(t *testing.T) {
		err := cm.CheckCircuit("test-service")
		assert.NoError(t, err)
	})

	t.Run("record success and failure", func(t *testing.T) {
		cm.RecordSuccess("test-service")
		cm.RecordFailure("test-service")
		state := cm.GetCircuitState("test-service")
		assert.Equal(t, StateClosed, state)
	})

	t.Run("no circuit breaker", func(t *testing.T) {
		cm2 := NewClientManager(nil, nil, nil, nil)
		err := cm2.CheckCircuit("test-service")
		assert.NoError(t, err)

		cm2.RecordSuccess("test-service") // Should not panic
		cm2.RecordFailure("test-service") // Should not panic
		state := cm2.GetCircuitState("test-service")
		assert.Equal(t, StateClosed, state)
	})
}

func TestClientManager_Watch(t *testing.T) {
	t.Run("no discovery configured", func(t *testing.T) {
		cm := NewClientManager(nil, nil, nil, nil)
		_, err := cm.Watch(context.Background(), "test-service")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "service discovery not configured")
	})

	t.Run("watch returns channel", func(t *testing.T) {
		ch := make(chan []*ServiceInstance)
		discovery := &MockDiscovery{
			WatchFunc: func(ctx context.Context, serviceName string) (<-chan []*ServiceInstance, error) {
				return ch, nil
			},
		}
		cm := NewClientManager(discovery, nil, nil, nil)

		result, err := cm.Watch(context.Background(), "test-service")
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})
}

func TestClientManager_Stop(t *testing.T) {
	stopped := false
	discovery := &MockDiscovery{
		StopFunc: func() {
			stopped = true
		},
	}
	cm := NewClientManager(discovery, nil, nil, zap.NewNop())

	cm.Stop()
	assert.True(t, stopped)
}

func TestClientManager_Stop_NoDiscovery(t *testing.T) {
	cm := NewClientManager(nil, nil, nil, zap.NewNop())
	cm.Stop() // Should not panic
}
