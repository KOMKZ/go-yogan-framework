package grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/KOMKZ/go-yogan-framework/component"
	"github.com/KOMKZ/go-yogan-framework/governance"
	"github.com/KOMKZ/go-yogan-framework/limiter"
	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/KOMKZ/go-yogan-framework/registry"
	"github.com/KOMKZ/go-yogan-framework/telemetry"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// Component gRPC 组件
type Component struct {
	server             *Server
	clientManager      *ClientManager
	registry           *registry.Registry // 🎯 使用具体类型，支持泛型方法
	log                *logger.CtxZapLogger
	config             Config                        // 保存配置用于后续注入选择器
	customInterceptors []grpc.UnaryServerInterceptor // 自定义拦截器
	limiter            *limiter.Manager              // 🎯 限速管理器（可选）
	tracerProvider     trace.TracerProvider          // 🎯 OpenTelemetry TracerProvider（可选）
}

// NewComponent 创建 gRPC 组件
func NewComponent() *Component {
	return &Component{}
}

// Name 组件名称
func (c *Component) Name() string {
	return component.ComponentGRPC
}

// DependsOn gRPC 组件依赖配置、日志、限流器，可选依赖 Telemetry
func (c *Component) DependsOn() []string {
	return []string{
		component.ComponentConfig,
		component.ComponentLogger,
		component.ComponentLimiter,
		"optional:" + component.ComponentTelemetry, // 🎯 可选依赖 Telemetry
		// 治理组件是可选依赖（如果存在则自动使用服务发现）
		// "optional:" + component.ComponentGovernance,
	}
}

// Init 初始化 gRPC 组件
func (c *Component) Init(ctx context.Context, loader component.ConfigLoader) error {
	// 🎯 统一在 Init 开始时保存 logger 到字段
	c.log = logger.GetLogger("yogan")

	// 1. 加载配置
	var cfg Config
	if err := loader.Unmarshal("grpc", &cfg); err != nil {
		return err
	}

	// 保存配置
	c.config = cfg

	// 2. 尝试获取 TracerProvider（可选，如果存在则在构建拦截器时使用）
	// 注意：Init 阶段 Telemetry 组件可能还未初始化，所以这里不获取
	// 将在 Start 阶段注入

	// 3. 初始化服务端（如果启用）- 🎯 统一使用 c.log
	if cfg.Server.Enabled {
		interceptors := c.buildInterceptorChain(cfg.Server, c.log)
		c.server = NewServerWithInterceptors(cfg.Server, c.log, interceptors)
	}

	// 4. 初始化客户端管理器（如果有配置）- 🎯 统一使用 c.log
	if len(cfg.Clients) > 0 {
		c.clientManager = NewClientManager(cfg.Clients, c.log)
	}

	return nil
}

// Start 启动 gRPC 组件（自动注入服务发现和负载均衡策略）
func (c *Component) Start(ctx context.Context) error {
	// 🎯 创建通用注入器
	injector := registry.NewInjector(c.registry, c.log)

	// 🎯 自动从 Telemetry 组件获取 TracerProvider 并注入
	c.injectTracerProvider(ctx, injector)

	// 🎯 自动从 Telemetry 组件获取 MetricsManager 并注入
	c.injectMetricsManager(ctx, injector)

	// 🎯 自动从治理组件获取服务发现器并注入
	if c.clientManager != nil {
		c.injectServiceDiscovery(ctx, injector)
		c.injectLoadBalancer(ctx)
		c.injectBreaker(ctx, injector) // 🎯 注入熔断器
		c.injectLimiter(ctx, injector) // 🎯 注入限速管理器

		// 自动预连接所有客户端
		c.clientManager.PreConnect(3 * time.Second)
		c.log.DebugCtx(ctx, "🔗 gRPC client pre-connection completed")
	}

	// gRPC Server 的启动由业务层在注册服务后手动调用 StartServer()
	return nil
}

// injectServiceDiscovery 从治理组件获取服务发现器并注入到客户端管理器
func (c *Component) injectServiceDiscovery(ctx context.Context, injector *registry.ComponentInjector) {
	registry.Inject(injector, ctx, component.ComponentGovernance,
		nil, // 无需额外检查
		func(govComp *governance.Component) {
			discovery := govComp.GetDiscovery()
			if discovery == nil {
				c.log.WarnCtx(ctx, "Governance component did not provide service discovery")
				return
			}

			// 注入服务发现器（类型断言为具体类型）
			etcdDiscovery, ok := discovery.(*governance.EtcdDiscovery)
			if !ok {
				c.log.ErrorCtx(ctx, "Service discovery type assertion failed, expected *governance.EtcdDiscovery")
				return
			}

			c.clientManager.SetDiscovery(etcdDiscovery)
			c.log.DebugCtx(ctx, "✅ Service discovery injected into gRPC client manager")
		},
	)
}

// injectLoadBalancer 根据配置注入负载均衡策略
func (c *Component) injectLoadBalancer(ctx context.Context) {
	// 🎯 策略：从第一个配置的客户端读取 load_balance 作为全局策略
	// 原因：保持简单，避免过度设计
	// 扩展：如需每个客户端独立策略，可修改为 map[serviceName]selector

	var strategy string
	for _, clientCfg := range c.config.Clients {
		if clientCfg.LoadBalance != "" {
			strategy = clientCfg.LoadBalance
			break
		}
	}

	if strategy == "" {
		// 未配置，使用默认策略（FirstHealthy）
		c.log.DebugCtx(ctx, "Load balancing strategy not configured, using default (select first healthy instance)")
		return
	}

	// 创建并注入选择器
	selector := NewInstanceSelector(strategy)
	c.clientManager.SetSelector(selector)
	c.log.DebugCtx(ctx, "✅ Load balancing strategy injected",
		zap.String("strategy", strategy))
}

// injectBreaker 从治理组件获取熔断器并注入到客户端管理器
func (c *Component) injectBreaker(ctx context.Context, injector *registry.ComponentInjector) {
	registry.Inject(injector, ctx, component.ComponentGovernance,
		nil, // 无需额外检查
		func(govComp *governance.Component) {
			breakerMgr := govComp.GetBreakerManager()
			if breakerMgr == nil {
				c.log.DebugCtx(ctx, "Circuit breaker not enabled, skipping injection")
				return
			}

			c.clientManager.SetBreaker(breakerMgr)
			c.log.DebugCtx(ctx, "✅ Circuit breaker injected from governance to gRPC client")
		},
	)
}

// injectLimiter 从 Limiter 组件获取限速管理器并注入到客户端管理器
func (c *Component) injectLimiter(ctx context.Context, injector *registry.ComponentInjector) {
	registry.Inject(injector, ctx, component.ComponentLimiter,
		nil, // 无需额外检查
		func(limiterComp *limiter.Component) {
			limiterMgr := limiterComp.GetManager()
			if limiterMgr == nil || !limiterMgr.IsEnabled() {
				c.log.DebugCtx(ctx, "Limiter manager not available or disabled")
				return
			}

			// 保存到 Component
			c.limiter = limiterMgr

			// 注入到客户端管理器
			c.clientManager.SetLimiter(limiterMgr)
			c.log.DebugCtx(ctx, "✅ Rate limiter injected into gRPC client manager")
		},
	)
}

// injectTracerProvider 从 Telemetry 组件获取 TracerProvider 并注入
func (c *Component) injectTracerProvider(ctx context.Context, injector *registry.ComponentInjector) {
	registry.Inject(injector, ctx, component.ComponentTelemetry,
		func(tc *telemetry.Component) bool { return tc.IsEnabled() },
		func(tc *telemetry.Component) {
			tp := tc.GetTracerProvider()
			if tp == nil {
				c.log.WarnCtx(ctx, "TracerProvider is nil")
				return
			}

			// 保存到 Component
			c.tracerProvider = tp

			// 注入到服务端
			if c.server != nil {
				c.server.SetTracerProvider(tp)
				c.log.DebugCtx(ctx, "✅ TracerProvider injected into gRPC server")
			}

			// 注入到客户端管理器
			if c.clientManager != nil {
				c.clientManager.SetTracerProvider(tp)
				c.log.DebugCtx(ctx, "✅ TracerProvider injected into gRPC client manager")
			}
		},
	)
}

// injectMetricsManager 从 Telemetry 组件获取 MetricsManager 并注入
func (c *Component) injectMetricsManager(ctx context.Context, injector *registry.ComponentInjector) {
	registry.Inject(injector, ctx, component.ComponentTelemetry,
		func(tc *telemetry.Component) bool {
			// 检查 Telemetry 启用 + MetricsManager 可用 + gRPC Metrics 启用
			if !tc.IsEnabled() {
				return false
			}
			mm := tc.GetMetricsManager()
			return mm != nil && mm.IsGRPCMetricsEnabled()
		},
		func(tc *telemetry.Component) {
			// 创建 gRPC Metrics（使用默认配置）
			grpcMetrics, err := NewGRPCMetrics(false, false)
			if err != nil {
				c.log.ErrorCtx(ctx, "Failed to create GRPCMetrics", zap.Error(err))
				return
			}

			// 注入到服务端
			if c.server != nil {
				c.server.SetMetricsHandler(grpcMetrics.StatsHandler())
				c.log.DebugCtx(ctx, "✅ Metrics StatsHandler injected into gRPC server")
			}

			// 注入到客户端管理器
			if c.clientManager != nil {
				c.clientManager.SetMetricsHandler(grpcMetrics.StatsHandler())
				c.log.DebugCtx(ctx, "✅ Metrics StatsHandler injected into gRPC client manager")
			}
		},
	)
}

// StartServer 手动启动 gRPC Server（在注册服务后调用）
func (c *Component) StartServer(ctx context.Context) error {
	if c.server != nil {
		if err := c.server.Start(ctx); err != nil {
			return fmt.Errorf("启动 gRPC Server 失败: %w", err)
		}

		// 🎯 启动成功后，自动注册到治理中心
		if err := c.registerToGovernance(ctx); err != nil {
			// 注册失败仅告警，不阻止应用启动
			c.log.WarnCtx(ctx, "⚠️  服务注册失败", zap.Error(err))
		}
	}
	return nil
}

// registerToGovernance 注册服务到治理中心（内部方法）
func (c *Component) registerToGovernance(ctx context.Context) error {
	if c.registry == nil {
		return nil
	}

	// 获取治理组件
	govComp, ok := registry.GetTyped[*governance.Component](c.registry, component.ComponentGovernance)
	if !ok {
		// 组件未注册，跳过
		return nil
	}

	// 调用治理组件的注册方法
	if err := govComp.RegisterService(c.server.Port); err != nil {
		return err
	}

	c.log.InfoCtx(ctx, "✅ 服务已注册到治理中心", zap.Int("port", c.server.Port))
	return nil
}

// Stop 停止 gRPC 组件
func (c *Component) Stop(ctx context.Context) error {
	// 1. 关闭服务端
	if c.server != nil {
		c.server.Stop(ctx)
	}

	// 2. 关闭客户端连接池
	if c.clientManager != nil {
		c.clientManager.Close()
	}

	return nil
}

// GetServer 获取 gRPC Server（业务层使用）
func (c *Component) GetServer() *Server {
	return c.server
}

// GetGRPCServer 便捷方法：直接获取原生 gRPC Server（用于注册服务）
func (c *Component) GetGRPCServer() *grpc.Server {
	if c.server == nil {
		return nil
	}
	return c.server.GetGRPCServer()
}

// GetClientManager 获取客户端管理器（业务层使用）
func (c *Component) GetClientManager() *ClientManager {
	return c.clientManager
}

// GetHealthChecker 获取健康检查器
// 实现 component.HealthCheckProvider 接口
func (c *Component) GetHealthChecker() component.HealthChecker {
	return NewHealthChecker(c.server, c.clientManager)
}

// SetRegistry 设置注册中心（由框架调用）
func (c *Component) SetRegistry(r *registry.Registry) {
	c.registry = r
}

// RegisterInterceptor 注册自定义 Unary 拦截器（应用层调用）
func (c *Component) RegisterInterceptor(interceptor grpc.UnaryServerInterceptor) {
	c.customInterceptors = append(c.customInterceptors, interceptor)
}

// ClearInterceptors 清空自定义拦截器（用于测试）
func (c *Component) ClearInterceptors() {
	c.customInterceptors = nil
}

// buildInterceptorChain 构建完整拦截器链
func (c *Component) buildInterceptorChain(
	cfg ServerConfig,
	log *logger.CtxZapLogger,
) []grpc.UnaryServerInterceptor {
	// 从配置读取是否启用日志（默认 true）
	enableLog := cfg.IsLogEnabled()

	// 内核拦截器链（不包括 OTel，已由 StatsHandler 处理）
	chain := []grpc.UnaryServerInterceptor{}

	// 1️⃣ TraceID 提取
	chain = append(chain, UnaryServerTraceInterceptor())

	// 2️⃣ 日志记录
	chain = append(chain, UnaryLoggerInterceptor(log, enableLog))

	// 自定义拦截器（中间）
	chain = append(chain, c.customInterceptors...)

	// 3️⃣ Panic 恢复（后置）
	chain = append(chain, UnaryRecoveryInterceptor(log))

	return chain
}
