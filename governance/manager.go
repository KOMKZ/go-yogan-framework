package governance

import (
	"context"
	"fmt"
	"sync"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"go.uber.org/zap"
)

// Manager Service governance manager
// Unified management of service registration, health checks, lifecycle, etc.
type Manager struct {
	// Service registry
	registry ServiceRegistry

	// Health checker
	healthChecker HealthChecker

	// service information
	serviceInfo *ServiceInfo

	// state management
	mu         sync.RWMutex
	registered bool

	// Log
	logger *logger.CtxZapLogger
}

// ManagerConfig Governance manager configuration (deprecated, use Config in component_config.go)
type ManagerConfig struct {
	// Registry service registration configuration
	Registry RegistryConfig `mapstructure:"registry"`

	// HealthCheck configuration
	HealthCheck HealthCheckConfig `mapstructure:"health_check"`
}

// RegistryConfig service registration configuration (deprecated)
type RegistryConfig struct {
	Enabled     bool              `mapstructure:"enabled"`      // Whether service registration is enabled
	Type        string            `mapstructure:"type"`         // Registry center type (etcd/consul/nacos)
	ServiceName string            `mapstructure:"service_name"` // service name
	TTL         int64             `mapstructure:"ttl"`          // Heartbeat interval (seconds)
	Metadata    map[string]string `mapstructure:"metadata"`     // metadata
}

// Create service governance manager
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

// Register service
// This is the main entry point called by the application framework
func (m *Manager) RegisterService(ctx context.Context, info *ServiceInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.registered {
		return ErrAlreadyRegistered
	}

	// Verify service information
	if err := info.Validate(); err != nil {
		return fmt.Errorf("validate service info: %w", err)
	}

	// Save service information
	m.serviceInfo = info

	// Call the registrar to register the service
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

// Unregister Service
func (m *Manager) DeregisterService(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.registered {
		return ErrNotRegistered
	}

	// Call the registrar to unregister the service
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

// UpdateMetadata Update service metadata
func (m *Manager) UpdateMetadata(ctx context.Context, metadata map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.registered {
		return ErrNotRegistered
	}

	// Update local metadata
	if m.serviceInfo.Metadata == nil {
		m.serviceInfo.Metadata = make(map[string]string)
	}
	for k, v := range metadata {
		m.serviceInfo.Metadata[k] = v
	}

	// Call the registry update
	if err := m.registry.UpdateMetadata(ctx, metadata); err != nil {
		return fmt.Errorf("update metadata: %w", err)
	}

	m.logger.DebugCtx(ctx, "✅ Service metadata updated", zap.Any("metadata", metadata))

	return nil
}

// PerformHealthCheck Execute health check
func (m *Manager) PerformHealthCheck(ctx context.Context) error {
	return m.healthChecker.Check(ctx)
}

// Get Health Status
func (m *Manager) GetHealthStatus() HealthStatus {
	return m.healthChecker.GetStatus()
}

// Checks if the service is registered
func (m *Manager) IsRegistered() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.registered
}

// Get service information
func (m *Manager) GetServiceInfo() *ServiceInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.serviceInfo
}

// Shut down governance manager (unregister service)
func (m *Manager) Shutdown(ctx context.Context) error {
	m.logger.DebugCtx(ctx, "🔻 Starting governance manager shutdown...")

	if err := m.DeregisterService(ctx); err != nil && err != ErrNotRegistered {
		return err
	}

	m.logger.DebugCtx(ctx, "✅ Governance manager closed")
	return nil
}
