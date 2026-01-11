package governance

import (
	"context"
	"fmt"
	"sync"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"go.uber.org/zap"
)

// Manager 服务治理管理器
// 统一管理服务注册、健康检查、生命周期等
type Manager struct {
	// 服务注册器
	registry ServiceRegistry

	// 健康检查器
	healthChecker HealthChecker

	// 服务信息
	serviceInfo *ServiceInfo

	// 状态管理
	mu         sync.RWMutex
	registered bool

	// 日志
	logger *logger.CtxZapLogger
}

// ManagerConfig 治理管理器配置（已废弃，使用 component_config.go 中的 Config）
type ManagerConfig struct {
	// Registry 服务注册配置
	Registry RegistryConfig `mapstructure:"registry"`

	// HealthCheck 健康检查配置
	HealthCheck HealthCheckConfig `mapstructure:"health_check"`
}

// RegistryConfig 服务注册配置（已废弃）
type RegistryConfig struct {
	Enabled     bool              `mapstructure:"enabled"`      // 是否启用服务注册
	Type        string            `mapstructure:"type"`         // 注册中心类型（etcd/consul/nacos）
	ServiceName string            `mapstructure:"service_name"` // 服务名称
	TTL         int64             `mapstructure:"ttl"`          // 心跳间隔（秒）
	Metadata    map[string]string `mapstructure:"metadata"`     // 元数据
}

// NewManager 创建服务治理管理器
func NewManager(registry ServiceRegistry, healthChecker HealthChecker, log *logger.CtxZapLogger) *Manager {
	if log == nil {
		log = logger.GetLogger("yogan")
	}

	if healthChecker == nil {
		healthChecker = NewDefaultHealthChecker()
	}

	return &Manager{
		registry:      registry,
		healthChecker: healthChecker,
		logger:        log,
	}
}

// RegisterService 注册服务
// 这是应用框架调用的主要入口点
func (m *Manager) RegisterService(ctx context.Context, info *ServiceInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.registered {
		return ErrAlreadyRegistered
	}

	// 验证服务信息
	if err := info.Validate(); err != nil {
		return fmt.Errorf("validate service info: %w", err)
	}

	// 保存服务信息
	m.serviceInfo = info

	// 调用注册器注册服务
	if err := m.registry.Register(ctx, info); err != nil {
		return fmt.Errorf("register service: %w", err)
	}

	m.registered = true

	m.logger.DebugCtx(ctx, "✅ Service registered",
		zap.String("service", info.ServiceName),
		zap.String("instance", info.InstanceID),
		zap.String("address", info.GetFullAddress()),
	)

	return nil
}

// DeregisterService 注销服务
func (m *Manager) DeregisterService(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.registered {
		return ErrNotRegistered
	}

	// 调用注册器注销服务
	if err := m.registry.Deregister(ctx); err != nil {
		m.logger.ErrorCtx(ctx, "Service deregistration failed", zap.Error(err))
		return fmt.Errorf("deregister service: %w", err)
	}

	m.registered = false

	m.logger.DebugCtx(ctx, "✅ Service deregistered",
		zap.String("service", m.serviceInfo.ServiceName),
		zap.String("instance", m.serviceInfo.InstanceID),
	)

	return nil
}

// UpdateMetadata 更新服务元数据
func (m *Manager) UpdateMetadata(ctx context.Context, metadata map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.registered {
		return ErrNotRegistered
	}

	// 更新本地元数据
	if m.serviceInfo.Metadata == nil {
		m.serviceInfo.Metadata = make(map[string]string)
	}
	for k, v := range metadata {
		m.serviceInfo.Metadata[k] = v
	}

	// 调用注册器更新
	if err := m.registry.UpdateMetadata(ctx, metadata); err != nil {
		return fmt.Errorf("update metadata: %w", err)
	}

	m.logger.DebugCtx(ctx, "✅ Service metadata updated", zap.Any("metadata", metadata))

	return nil
}

// PerformHealthCheck 执行健康检查
func (m *Manager) PerformHealthCheck(ctx context.Context) error {
	return m.healthChecker.Check(ctx)
}

// GetHealthStatus 获取健康状态
func (m *Manager) GetHealthStatus() HealthStatus {
	return m.healthChecker.GetStatus()
}

// IsRegistered 检查服务是否已注册
func (m *Manager) IsRegistered() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.registered
}

// GetServiceInfo 获取服务信息
func (m *Manager) GetServiceInfo() *ServiceInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.serviceInfo
}

// Shutdown 关闭治理管理器（注销服务）
func (m *Manager) Shutdown(ctx context.Context) error {
	m.logger.DebugCtx(ctx, "🔻 Starting governance manager shutdown...")

	if err := m.DeregisterService(ctx); err != nil && err != ErrNotRegistered {
		return err
	}

	m.logger.DebugCtx(ctx, "✅ Governance manager closed")
	return nil
}
