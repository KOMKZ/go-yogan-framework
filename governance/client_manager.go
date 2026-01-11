package governance

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

// ClientManager 客户端治理管理器（服务发现 + 负载均衡 + 熔断）
type ClientManager struct {
	discovery      ServiceDiscovery
	loadBalancer   LoadBalancer
	circuitBreaker CircuitBreaker
	logger         *zap.Logger
}

// ClientConfig 客户端治理配置
type ClientConfig struct {
	DiscoveryEnabled bool                 `mapstructure:"discovery_enabled"` // 是否启用服务发现
	LoadBalance      string               `mapstructure:"load_balance"`      // 负载均衡策略
	CircuitBreaker   CircuitBreakerConfig `mapstructure:"circuit_breaker"`   // 熔断器配置
}

// NewClientManager 创建客户端治理管理器
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

// Discover 发现服务实例
func (m *ClientManager) Discover(ctx context.Context, serviceName string) ([]*ServiceInstance, error) {
	if m.discovery == nil {
		return nil, fmt.Errorf("service discovery not configured")
	}

	return m.discovery.Discover(ctx, serviceName)
}

// SelectInstance 选择一个服务实例（发现 + 负载均衡）
func (m *ClientManager) SelectInstance(ctx context.Context, serviceName string) (*ServiceInstance, error) {
	// 1. 服务发现
	instances, err := m.Discover(ctx, serviceName)
	if err != nil {
		return nil, fmt.Errorf("discover service failed: %w", err)
	}

	if len(instances) == 0 {
		return nil, fmt.Errorf("no available instances for service: %s", serviceName)
	}

	// 2. 过滤健康实例
	healthyInstances := make([]*ServiceInstance, 0, len(instances))
	for _, inst := range instances {
		if inst.Healthy {
			healthyInstances = append(healthyInstances, inst)
		}
	}

	if len(healthyInstances) == 0 {
		m.logger.Warn("所有实例均不健康，降级使用全部实例",
			zap.String("service", serviceName))
		healthyInstances = instances
	}

	// 3. 负载均衡选择
	if m.loadBalancer == nil {
		return healthyInstances[0], nil
	}

	instance := m.loadBalancer.Select(healthyInstances)
	if instance == nil {
		return nil, fmt.Errorf("load balancer returned nil for service: %s", serviceName)
	}

	return instance, nil
}

// CheckCircuit 检查熔断状态
func (m *ClientManager) CheckCircuit(serviceName string) error {
	if m.circuitBreaker == nil {
		return nil // 未启用熔断器
	}

	state := m.circuitBreaker.GetState(serviceName)
	if state == StateOpen {
		return fmt.Errorf("circuit breaker is open for service: %s", serviceName)
	}

	return nil
}

// RecordSuccess 记录调用成功
func (m *ClientManager) RecordSuccess(serviceName string) {
	if m.circuitBreaker != nil {
		m.circuitBreaker.RecordSuccess(serviceName)
	}
}

// RecordFailure 记录调用失败
func (m *ClientManager) RecordFailure(serviceName string) {
	if m.circuitBreaker != nil {
		m.circuitBreaker.RecordFailure(serviceName)
	}
}

// GetCircuitState 获取熔断状态
func (m *ClientManager) GetCircuitState(serviceName string) CircuitState {
	if m.circuitBreaker == nil {
		return StateClosed
	}

	return m.circuitBreaker.GetState(serviceName)
}

// Watch 监听服务变更
func (m *ClientManager) Watch(ctx context.Context, serviceName string) (<-chan []*ServiceInstance, error) {
	if m.discovery == nil {
		return nil, fmt.Errorf("service discovery not configured")
	}

	return m.discovery.Watch(ctx, serviceName)
}

// Stop 停止客户端治理
func (m *ClientManager) Stop() {
	if m.discovery != nil {
		m.discovery.Stop()
	}
	m.logger.Debug("✅ 客户端治理管理器已停止")
}
