package governance

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

// ClientManager client governance manager (service discovery + load balancing + circuit breaking)
type ClientManager struct {
	discovery      ServiceDiscovery
	loadBalancer   LoadBalancer
	circuitBreaker CircuitBreaker
	logger         *zap.Logger
}

// ClientConfig client governance configuration
type ClientConfig struct {
	DiscoveryEnabled bool                 `mapstructure:"discovery_enabled"` // Enable service discovery
	LoadBalance      string               `mapstructure:"load_balance"`      // load balancing strategy
	CircuitBreaker   CircuitBreakerConfig `mapstructure:"circuit_breaker"`   // Circuit breaker configuration
}

// Create client governance manager
func NewClientManager(
	discovery ServiceDiscovery,
	loadBalancer LoadBalancer,
	circuitBreaker CircuitBreaker,
	logger *zap.Logger,
) *ClientManager {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &ClientManager{
		discovery:      discovery,
		loadBalancer:   loadBalancer,
		circuitBreaker: circuitBreaker,
		logger:         logger,
	}
}

// Discover service instances
func (m *ClientManager) Discover(ctx context.Context, serviceName string) ([]*ServiceInstance, error) {
	if m.discovery == nil {
		return nil, fmt.Errorf("service discovery not configured")
	}

	return m.discovery.Discover(ctx, serviceName)
}

// SelectInstance selects a service instance (discovery + load balancing)
func (m *ClientManager) SelectInstance(ctx context.Context, serviceName string) (*ServiceInstance, error) {
	// Service discovery
	instances, err := m.Discover(ctx, serviceName)
	if err != nil {
		return nil, fmt.Errorf("discover service failed: %w", err)
	}

	if len(instances) == 0 {
		return nil, fmt.Errorf("no available instances for service: %s", serviceName)
	}

	// Filter healthy instances
	healthyInstances := make([]*ServiceInstance, 0, len(instances))
	for _, inst := range instances {
		if inst.Healthy {
			healthyInstances = append(healthyInstances, inst)
		}
	}

	if len(healthyInstances) == 0 {
		m.logger.Warn("All instances are unhealthy, downgrading to use all instances，All instances are unhealthy, downgrading to use all instances",
			zap.String("service", serviceName))
		healthyInstances = instances
	}

	// Load balancing selection
	if m.loadBalancer == nil {
		return healthyInstances[0], nil
	}

	instance := m.loadBalancer.Select(healthyInstances)
	if instance == nil {
		return nil, fmt.Errorf("load balancer returned nil for service: %s", serviceName)
	}

	return instance, nil
}

// CheckCircuit checks the circuit breaker status
func (m *ClientManager) CheckCircuit(serviceName string) error {
	if m.circuitBreaker == nil {
		return nil // Circuit breaker not enabled
	}

	state := m.circuitBreaker.GetState(serviceName)
	if state == StateOpen {
		return fmt.Errorf("circuit breaker is open for service: %s", serviceName)
	}

	return nil
}

// RecordSuccess record call success
func (m *ClientManager) RecordSuccess(serviceName string) {
	if m.circuitBreaker != nil {
		m.circuitBreaker.RecordSuccess(serviceName)
	}
}

// RecordFailure record call failure
func (m *ClientManager) RecordFailure(serviceName string) {
	if m.circuitBreaker != nil {
		m.circuitBreaker.RecordFailure(serviceName)
	}
}

// GetCircuitState get circuit breaker state
func (m *ClientManager) GetCircuitState(serviceName string) CircuitState {
	if m.circuitBreaker == nil {
		return StateClosed
	}

	return m.circuitBreaker.GetState(serviceName)
}

// Watch for service changes
func (m *ClientManager) Watch(ctx context.Context, serviceName string) (<-chan []*ServiceInstance, error) {
	if m.discovery == nil {
		return nil, fmt.Errorf("service discovery not configured")
	}

	return m.discovery.Watch(ctx, serviceName)
}

// Stop client governance
func (m *ClientManager) Stop() {
	if m.discovery != nil {
		m.discovery.Stop()
	}
	m.logger.Debug("✅ ✅ Client governance manager has stopped")
}
