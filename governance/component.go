package governance

import (
	"context"
	"fmt"
	"time"

	"github.com/KOMKZ/go-yogan-framework/breaker"
	"github.com/KOMKZ/go-yogan-framework/component"
	"github.com/KOMKZ/go-yogan-framework/logger"
	"go.uber.org/zap"
)

// Component 治理组件（标准组件）
type Component struct {
	config  *Config
	manager *Manager
	logger  *logger.CtxZapLogger // 使用 CtxZapLogger

	// breaker管理器
	breakerMgr *breaker.Manager

	// 内部状态
	registered bool

	// 保存 ConfigLoader 供 Start 使用
	configLoader component.ConfigLoader

	// 🎯 服务发现器（供客户端使用）
	discovery ServiceDiscovery
}

// NewComponent 创建治理组件
func NewComponent() *Component {
	return &Component{}
}

// Name 组件名称
func (c *Component) Name() string {
	return component.ComponentGovernance
}

// DependsOn 声明依赖（无依赖）
func (c *Component) DependsOn() []string {
	return []string{
		component.ComponentConfig,
		component.ComponentLogger,
	}
}

// Init 初始化组件（框架自动调用）
func (c *Component) Init(ctx context.Context, loader component.ConfigLoader) error {
	c.configLoader = loader

	// 🎯 依赖注入：使用 CtxZapLogger
	c.logger = logger.GetLogger("yogan")

	// 加载配置
	var cfg Config
	if err := loader.Unmarshal("governance", &cfg); err != nil {
		return fmt.Errorf("加载治理配置失败: %w", err)
	}

	c.logger.DebugCtx(ctx, "🔍 [DEBUG] Governance config loaded",
		zap.Bool("enabled", cfg.Enabled),
		zap.Bool("breaker_enabled", cfg.Breaker.Enabled),
		zap.Int("event_bus_buffer", cfg.Breaker.EventBusBuffer))

	// 如果未启用，跳过初始化
	if !cfg.Enabled {
		c.logger.DebugCtx(ctx, "⏭️  Governance component not enabled")
		return nil
	}

	c.config = &cfg

	// 创建注册器（用于服务注册）
	serviceRegistry, err := c.createRegistry(ctx, &cfg)
	if err != nil {
		return fmt.Errorf("创建注册器失败: %w", err)
	}

	// 🎯 创建服务发现器（供客户端使用）
	c.discovery, err = c.createDiscovery(ctx, &cfg)
	if err != nil {
		return fmt.Errorf("创建服务发现器失败: %w", err)
	}

	// 创建治理管理器
	c.manager = NewManager(serviceRegistry, nil, c.logger)

	// 初始化熔断器管理器（如果配置了）
	if err := c.initBreaker(ctx, &cfg); err != nil {
		return fmt.Errorf("初始化熔断器失败: %w", err)
	}

	c.logger.DebugCtx(ctx, "✅ Governance component initialized",
		zap.String("registry_type", cfg.RegistryType),
		zap.String("service_name", cfg.ServiceName),
	)

	return nil
}

// Start 启动组件（空实现，不做自动注册）
func (c *Component) Start(ctx context.Context) error {
	// 🎯 不在这里自动注册，等待应用层显式调用
	return nil
}

// Stop 停止组件（自动注销服务）
func (c *Component) Stop(ctx context.Context) error {
	if c.manager == nil || !c.registered {
		return nil
	}

	c.logger.DebugCtx(ctx, "🔻 Starting service deregistration...")

	if err := c.manager.Shutdown(ctx); err != nil {
		c.logger.ErrorCtx(ctx, "Service deregistration failed", zap.Error(err))
		return err
	}

	c.logger.DebugCtx(ctx, "✅ Service deregistered")
	c.registered = false

	return nil
}

// RegisterService 注册服务（🎯 应用层显式调用）
// port: gRPC 服务端口
func (c *Component) RegisterService(port int) error {
	ctx := context.Background()

	if c.manager == nil {
		return fmt.Errorf("治理组件未初始化或未启用")
	}

	if c.registered {
		return fmt.Errorf("服务已注册")
	}

	// 构建服务信息
	serviceInfo := c.buildServiceInfo(port)

	// 注册服务
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := c.manager.RegisterService(timeoutCtx, serviceInfo); err != nil {
		return fmt.Errorf("注册服务失败: %w", err)
	}

	c.registered = true

	c.logger.DebugCtx(ctx, "✅ Service registered",
		zap.String("service", serviceInfo.ServiceName),
		zap.String("address", serviceInfo.GetFullAddress()),
		zap.Int64("ttl", serviceInfo.TTL),
	)

	return nil
}

// DeregisterService 注销服务（手动注销）
func (c *Component) DeregisterService() error {
	ctx := context.Background()

	if c.manager == nil || !c.registered {
		return nil
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := c.manager.DeregisterService(timeoutCtx); err != nil {
		return fmt.Errorf("注销服务失败: %w", err)
	}

	c.registered = false
	c.logger.DebugCtx(ctx, "✅ Service deregistered")

	return nil
}

// UpdateMetadata 更新服务元数据
func (c *Component) UpdateMetadata(metadata map[string]string) error {
	ctx := context.Background()

	if c.manager == nil {
		return fmt.Errorf("治理组件未初始化")
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return c.manager.UpdateMetadata(timeoutCtx, metadata)
}

// IsRegistered 检查服务是否已注册
func (c *Component) IsRegistered() bool {
	return c.registered
}

// GetManager 获取治理管理器（高级用法）
func (c *Component) GetManager() *Manager {
	return c.manager
}

// GetDiscovery 获取服务发现器（供客户端使用）
func (c *Component) GetDiscovery() ServiceDiscovery {
	return c.discovery
}

// GetBreakerManager 获取熔断器管理器
func (c *Component) GetBreakerManager() *breaker.Manager {
	return c.breakerMgr
}

// initBreaker 初始化熔断器
func (c *Component) initBreaker(ctx context.Context, cfg *Config) error {
	c.logger.DebugCtx(ctx, "🔍 [DEBUG] initBreaker started",
		zap.Bool("enabled", cfg.Breaker.Enabled),
		zap.Int("buffer", cfg.Breaker.EventBusBuffer),
		zap.Int("resources_count", len(cfg.Breaker.Resources)))

	// 检查breaker配置是否存在
	if !cfg.Breaker.Enabled {
		c.logger.DebugCtx(ctx, "🔍 [DEBUG] Breaker not enabled")
		return nil
	}

	// 🎯 初始化熔断器管理器
	var err error
	c.breakerMgr, err = breaker.NewManagerWithLogger(cfg.Breaker, c.logger)
	if err != nil {
		c.logger.ErrorCtx(ctx, "❌ Failed to initialize circuit breaker", zap.Error(err))
		return fmt.Errorf("初始化熔断器失败: %w", err)
	}

	c.logger.DebugCtx(ctx, "🔍 [DEBUG] breakerMgr created", zap.Bool("is_nil", c.breakerMgr == nil))

	c.subscribeBreakerEvents() // 订阅熔断器事件
	c.logger.DebugCtx(ctx, "✅ Circuit breaker manager initialized")

	return nil
}

// subscribeBreakerEvents 订阅熔断器事件并打印日志
func (c *Component) subscribeBreakerEvents() {
	if c.breakerMgr == nil {
		return
	}

	eventBus := c.breakerMgr.GetEventBus()
	if eventBus == nil {
		return
	}

	// 订阅所有事件
	eventBus.Subscribe(breaker.EventListenerFunc(func(event breaker.Event) {
		ctx := event.Context()
		resource := event.Resource()

		switch e := event.(type) {
		case *breaker.StateChangedEvent:
			// 状态变化事件 - 使用 Warn 级别
			c.logger.WarnCtx(ctx, "🔄 Circuit breaker state changed",
				zap.String("resource", resource),
				zap.String("from_state", e.FromState.String()),
				zap.String("to_state", e.ToState.String()),
				zap.String("reason", e.Reason),
				zap.Int64("total_requests", e.Metrics.TotalRequests),
				zap.Float64("error_rate", e.Metrics.ErrorRate),
			)

		case *breaker.CallEvent:
			// 调用事件 - 根据成功/失败使用不同级别
			if event.Type() == breaker.EventCallFailure {
				c.logger.ErrorCtx(ctx, "❌ Circuit breaker call failed",
					zap.String("resource", resource),
					zap.Duration("duration", e.Duration),
					zap.Error(e.Error),
				)
			} else if event.Type() == breaker.EventCallTimeout {
				c.logger.WarnCtx(ctx, "⏱️  Circuit breaker call timeout",
					zap.String("resource", resource),
					zap.Duration("duration", e.Duration),
				)
			}

		case *breaker.RejectedEvent:
			// 拒绝事件 - 使用 Warn 级别
			c.logger.WarnCtx(ctx, "🚫 Circuit breaker rejected request",
				zap.String("resource", resource),
				zap.String("current_state", e.CurrentState.String()),
			)

		case *breaker.FallbackEvent:
			// 降级事件
			if e.Success {
				c.logger.DebugCtx(ctx, "🔄 Circuit breaker fallback succeeded",
					zap.String("resource", resource),
					zap.Duration("duration", e.Duration),
				)
			} else {
				c.logger.ErrorCtx(ctx, "❌ Circuit breaker fallback failed",
					zap.String("resource", resource),
					zap.Duration("duration", e.Duration),
					zap.Error(e.Error),
				)
			}
		}
	}))

	c.logger.DebugCtx(context.Background(), "✅ Breaker events subscribed")
}

// buildServiceInfo 构建服务信息
func (c *Component) buildServiceInfo(port int) *ServiceInfo {
	cfg := c.config

	// 获取服务地址
	address := cfg.Address
	if address == "" {
		localIP, _ := GetLocalIP()
		address = localIP
	}

	// 复制元数据
	metadata := make(map[string]string)
	for k, v := range cfg.Metadata {
		metadata[k] = v
	}

	return &ServiceInfo{
		ServiceName: cfg.ServiceName,
		Address:     address,
		Port:        port,
		Protocol:    cfg.Protocol,
		Version:     cfg.Version,
		TTL:         cfg.TTL,
		Metadata:    metadata,
	}
}

// createRegistry 根据配置创建注册器
func (c *Component) createRegistry(ctx context.Context, cfg *Config) (ServiceRegistry, error) {
	switch cfg.RegistryType {
	case "etcd":
		// 为 EtcdRegistry 创建专用 logger
		etcdLogger := logger.GetLogger("yogan")
		// 直接传递完整配置（包含重试策略）
		return NewEtcdRegistry(cfg.Etcd, etcdLogger)

	default:
		return nil, fmt.Errorf("不支持的注册中心类型: %s", cfg.RegistryType)
	}
}

// createDiscovery 根据配置创建服务发现器
func (c *Component) createDiscovery(ctx context.Context, cfg *Config) (ServiceDiscovery, error) {
	switch cfg.RegistryType {
	case "etcd":

		// 🎯 创建 etcd 客户端（用于服务发现）
		etcdCfg := etcdClientConfig{
			Endpoints:   cfg.Etcd.Endpoints,
			DialTimeout: cfg.Etcd.DialTimeout,
		}
		etcdClient, err := newEtcdClient(etcdCfg, c.logger)
		if err != nil {
			return nil, fmt.Errorf("创建etcd客户端失败: %w", err)
		}

		return NewEtcdDiscovery(etcdClient, c.logger), nil

	default:
		return nil, fmt.Errorf("不支持的注册中心类型: %s", cfg.RegistryType)
	}
}
