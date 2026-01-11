package grpc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/KOMKZ/go-yogan-framework/breaker"
	"github.com/KOMKZ/go-yogan-framework/governance"
	"github.com/KOMKZ/go-yogan-framework/limiter"
	"github.com/KOMKZ/go-yogan-framework/logger"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ClientManager gRPC 客户端连接池管理器（支持服务发现）
type ClientManager struct {
	configs        map[string]ClientConfig
	conns          map[string]*grpc.ClientConn
	timeouts       map[string]time.Duration // 每个客户端的超时配置
	mu             sync.RWMutex
	logger         *logger.CtxZapLogger
	discovery      *governance.EtcdDiscovery // 服务发现器（可选）
	selector       InstanceSelector          // 实例选择器（可选，默认 FirstHealthy）
	breaker        *breaker.Manager          // 熔断器（可选）
	limiter        *limiter.Manager          // 🎯 限速管理器（可选）
	tracerProvider trace.TracerProvider      // 🎯 OpenTelemetry TracerProvider（可选）
	// Watch相关
	watchCtx    context.Context
	watchCancel context.CancelFunc
	watchWg     sync.WaitGroup
}

// NewClientManager 创建客户端管理器
func NewClientManager(configs map[string]ClientConfig, log *logger.CtxZapLogger) *ClientManager {
	ctx, cancel := context.WithCancel(context.Background())

	// 预计算每个客户端的超时时间
	timeouts := make(map[string]time.Duration)
	for name, cfg := range configs {
		timeouts[name] = time.Duration(cfg.GetTimeout()) * time.Second
	}

	return &ClientManager{
		configs:     configs,
		conns:       make(map[string]*grpc.ClientConn),
		timeouts:    timeouts,
		logger:      log,
		watchCtx:    ctx,
		watchCancel: cancel,
	}
}

// SetDiscovery 设置服务发现器（组件层注入）
func (m *ClientManager) SetDiscovery(discovery *governance.EtcdDiscovery) {
	m.discovery = discovery
}

// SetSelector 设置实例选择器（可选，默认 FirstHealthy）
func (m *ClientManager) SetSelector(selector InstanceSelector) {
	m.selector = selector
}

// SetBreaker 设置熔断器（由 gRPC 组件在 Start 时注入）
func (m *ClientManager) SetBreaker(b *breaker.Manager) {
	m.breaker = b
	ctx := context.Background()
	if b != nil {
		m.logger.DebugCtx(ctx, "✅ Circuit breaker injected into gRPC client manager")
	}
}

// GetBreaker 获取熔断器
func (m *ClientManager) GetBreaker() *breaker.Manager {
	return m.breaker
}

// SetLimiter 设置限速管理器（由 gRPC 组件在 Start 时注入）
func (m *ClientManager) SetLimiter(lim *limiter.Manager) {
	m.limiter = lim
	ctx := context.Background()
	if lim != nil && lim.IsEnabled() {
		m.logger.DebugCtx(ctx, "✅ Rate limiter injected into gRPC client manager")
	}
}

// SetTracerProvider 设置 TracerProvider
func (m *ClientManager) SetTracerProvider(tp trace.TracerProvider) {
	m.tracerProvider = tp
	ctx := context.Background()
	if tp != nil {
		m.logger.DebugCtx(ctx, "✅ TracerProvider injected into gRPC client manager")
	}
}

// SetMetricsHandler 设置 Metrics StatsHandler
// 注意：当前实现会在连接创建时使用，需要在 PreConnect 之前调用
func (m *ClientManager) SetMetricsHandler(handler interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 暂时不存储 handler，因为客户端的 Metrics 通过 otelgrpc.NewClientHandler 集成
	// 这里只是为了接口兼容性
	ctx := context.Background()
	m.logger.DebugCtx(ctx, "✅ Metrics StatsHandler set in ClientManager (placeholder)")
}

// GetLimiter 获取限速管理器
func (m *ClientManager) GetLimiter() *limiter.Manager {
	return m.limiter
}

// getSelector 获取选择器（带默认值）
func (m *ClientManager) getSelector() InstanceSelector {
	if m.selector == nil {
		return NewFirstHealthySelector() // 默认策略
	}
	return m.selector
}

// PreConnect 异步预连接所有配置的客户端（支持服务发现和直连）
func (m *ClientManager) PreConnect(timeout time.Duration) {
	ctx := context.Background()
	if len(m.configs) == 0 {
		return
	}

	m.logger.DebugCtx(ctx, "🔗 Starting gRPC client pre-connection...",
		zap.Int("count", len(m.configs)),
		zap.Duration("timeout", timeout))

	var wg sync.WaitGroup
	for serviceName, cfg := range m.configs {
		wg.Add(1)
		go func(name string, config ClientConfig) {
			defer wg.Done()

			// 🎯 根据配置选择连接模式
			if config.DiscoveryMode != "" && config.ServiceName != "" {
				m.preConnectWithDiscovery(name, config, timeout)
			} else {
				m.preConnectDirect(name, config, timeout)
			}
		}(serviceName, cfg)
	}

	// 等待所有连接完成（或超时）
	wg.Wait()
	m.logger.DebugCtx(ctx, "🔗 Pre-connection completed",
		zap.Int("conns", len(m.conns)),
		zap.Int("total", len(m.configs)))
}

// ========================================
// 公共方法：消除重复代码（DRY原则）
// ========================================

// discoverHealthyInstance 发现并选择健康实例
// 返回：实例地址，错误信息
func (m *ClientManager) discoverHealthyInstance(ctx context.Context, serviceName string) (string, error) {
	if m.discovery == nil {
		return "", fmt.Errorf("服务发现未初始化")
	}

	instances, err := m.discovery.Discover(ctx, serviceName)
	if err != nil {
		return "", fmt.Errorf("服务发现查询失败: %w", err)
	}

	if len(instances) == 0 {
		return "", fmt.Errorf("未发现服务实例: %s", serviceName)
	}

	// 使用注入的选择器选择实例
	selected := m.getSelector().Select(instances)
	if selected == nil {
		return "", fmt.Errorf("没有健康的服务实例: %s", serviceName)
	}

	return selected.GetAddress(), nil
}

// dialWithOptions 建立 gRPC 连接（复用拨号逻辑）
func (m *ClientManager) dialWithOptions(ctx context.Context, serviceName, targetAddr string, cfg ClientConfig) (*grpc.ClientConn, error) {
	// 创建客户端拦截器专用的 logger
	clientLogger := logger.GetLogger("yogan")
	enableLog := cfg.IsLogEnabled()

	// 获取超时配置
	timeout := m.timeouts[serviceName]

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(), // 阻塞等待连接成功
	}

	// 🎯 1. 添加 StatsHandler（优先级最高，用于 OpenTelemetry）
	if m.tracerProvider != nil {
		opts = append(opts, grpc.WithStatsHandler(
			otelgrpc.NewClientHandler(
				otelgrpc.WithTracerProvider(m.tracerProvider),
			),
		))
	}

	// 🎯 2. 构建拦截器链（不包括 OTel，已由 StatsHandler 处理）
	interceptors := []grpc.UnaryClientInterceptor{
		UnaryClientTraceInterceptor(),                         // 1️⃣ TraceID 传播
		UnaryClientRateLimitInterceptor(m, serviceName),       // 2️⃣ 限速检查
		UnaryClientBreakerInterceptor(m, serviceName),         // 3️⃣ 熔断器
		UnaryClientTimeoutInterceptor(timeout, clientLogger),  // 4️⃣ 超时控制
		UnaryClientLoggerInterceptor(clientLogger, enableLog), // 5️⃣ 日志记录（可配置）
	}
	opts = append(opts, grpc.WithChainUnaryInterceptor(interceptors...))

	// 3. 服务发现模式添加负载均衡配置
	if cfg.LoadBalance != "" {
		opts = append(opts, grpc.WithDefaultServiceConfig(
			fmt.Sprintf(`{"loadBalancingPolicy":"%s"}`, cfg.LoadBalance)))
	}

	return grpc.DialContext(ctx, targetAddr, opts...)
}

// preConnectWithDiscovery 服务发现模式预连接
// ✅ 重构后：Watch 监听独立启动，预连接尽力而为
func (m *ClientManager) preConnectWithDiscovery(serviceName string, cfg ClientConfig, timeout time.Duration) {
	// ✅ 第一步：无条件启动 Watch 监听（独立生命周期）
	m.startWatchForever(serviceName, cfg)

	// ✅ 第二步：尝试预连接（尽力而为，失败不影响 Watch）
	m.tryPreConnect(serviceName, cfg, timeout)
}

// tryPreConnect 尝试预连接（单一职责：连接建立）
func (m *ClientManager) tryPreConnect(serviceName string, cfg ClientConfig, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 1. 发现健康实例
	targetAddr, err := m.discoverHealthyInstance(ctx, cfg.ServiceName)
	if err != nil {
		m.logger.WarnCtx(ctx, "⚠️  Pre-connection failed (service discovery), will auto-retry at runtime",
			zap.String("service", serviceName),
			zap.String("target_service", cfg.ServiceName),
			zap.Error(err))
		return
	}

	// 2. 建立连接
	conn, err := m.dialWithOptions(ctx, serviceName, targetAddr, cfg)
	if err != nil {
		m.logger.WarnCtx(ctx, "⚠️  Pre-connection failed (connection establishment), will auto-retry at runtime",
			zap.String("service", serviceName),
			zap.String("target", targetAddr),
			zap.Error(err))
		return
	}

	// 3. 缓存连接
	m.mu.Lock()
	m.conns[serviceName] = conn
	m.mu.Unlock()

	m.logger.DebugCtx(ctx, "✅ Pre-connection succeeded (service discovery mode)",
		zap.String("service", serviceName),
		zap.String("target_service", cfg.ServiceName),
		zap.String("target", targetAddr),
		zap.String("load_balance", cfg.LoadBalance))
}

// startWatchForever 启动 Watch 监听（永不放弃，自动重试）
func (m *ClientManager) startWatchForever(serviceName string, cfg ClientConfig) {
	if m.discovery == nil || cfg.ServiceName == "" {
		return
	}

	m.watchWg.Add(1)
	go func() {
		defer m.watchWg.Done()

		backoff := time.Second
		maxBackoff := 30 * time.Second

		for {
			select {
			case <-m.watchCtx.Done():
				return
			default:
				// 尝试启动 Watch 循环
				err := m.runWatchLoop(serviceName, cfg)
				if err != nil {
					m.logger.WarnCtx(context.Background(),
						"⚠️  Watch interrupted, will retry later",
						zap.String("service", serviceName),
						zap.String("target_service", cfg.ServiceName),
						zap.Error(err),
						zap.Duration("retry_after", backoff))

					// 指数退避重试
					select {
					case <-m.watchCtx.Done():
						return
					case <-time.After(backoff):
						backoff = min(backoff*2, maxBackoff)
					}
				} else {
					// 正常退出，重置退避
					backoff = time.Second
				}
			}
		}
	}()
}

// runWatchLoop 执行一次 Watch 循环（单一职责）
func (m *ClientManager) runWatchLoop(serviceName string, cfg ClientConfig) error {
	ctx := context.Background()

	watchCh, err := m.discovery.Watch(ctx, cfg.ServiceName)
	if err != nil {
		return fmt.Errorf("启动Watch失败: %w", err)
	}

	m.logger.DebugCtx(ctx, "🔍 Service instance watch started",
		zap.String("service", serviceName),
		zap.String("target_service", cfg.ServiceName))

	for {
		select {
		case <-m.watchCtx.Done():
			return nil // 正常退出

		case instances, ok := <-watchCh:
			if !ok {
				return fmt.Errorf("Watch通道关闭")
			}

			// 处理实例更新
			m.handleInstancesUpdate(serviceName, cfg, instances)
		}
	}
}

// preConnectDirect 直连模式预连接
func (m *ClientManager) preConnectDirect(serviceName string, cfg ClientConfig, timeout time.Duration) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 使用 dialWithOptions 统一创建连接
	conn, err := m.dialWithOptions(ctx, serviceName, cfg.Target, cfg)
	if err != nil {
		m.logger.ErrorCtx(ctx, "❌ Pre-connection failed (service may be unavailable, will retry at runtime)",
			zap.String("service", serviceName),
			zap.String("target", cfg.Target),
			zap.Error(err),
			zap.Stack("stack"))
		return
	}

	// 缓存连接
	m.mu.Lock()
	m.conns[serviceName] = conn
	m.mu.Unlock()

	m.logger.DebugCtx(ctx, "✅ Pre-connection succeeded (direct mode)",
		zap.String("service", serviceName),
		zap.String("target", cfg.Target))
}

// GetConn 获取客户端连接（运行时调用）
func (m *ClientManager) GetConn(serviceName string) (*grpc.ClientConn, error) {
	// 检查配置是否存在
	cfg, ok := m.configs[serviceName]
	if !ok {
		return nil, fmt.Errorf("未配置服务: %s", serviceName)
	}

	m.mu.RLock()
	conn, exists := m.conns[serviceName]
	m.mu.RUnlock()

	if exists {
		return conn, nil
	}

	// 🎯 运行时动态连接（如果预连接失败）
	return m.connectOnDemand(serviceName, cfg)
}

// connectOnDemand 按需连接（运行时重试）
// ✅ 重构后：复用公共逻辑
func (m *ClientManager) connectOnDemand(serviceName string, cfg ClientConfig) (*grpc.ClientConn, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 双重检查
	if conn, exists := m.conns[serviceName]; exists {
		return conn, nil
	}

	// 使用配置的超时时间
	timeout := time.Duration(cfg.GetTimeout()) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var targetAddr string
	var err error

	// 🎯 服务发现模式：复用 discoverHealthyInstance
	if cfg.DiscoveryMode != "" && cfg.ServiceName != "" && m.discovery != nil {
		targetAddr, err = m.discoverHealthyInstance(ctx, cfg.ServiceName)
		if err != nil {
			return nil, fmt.Errorf("服务发现失败: %w", err)
		}
	} else {
		// 直连模式
		targetAddr = cfg.Target
	}

	// ✅ 复用 dialWithOptions 建立连接
	conn, err := m.dialWithOptions(ctx, serviceName, targetAddr, cfg)
	if err != nil {
		return nil, fmt.Errorf("连接失败: %w", err)
	}

	// 缓存连接
	m.conns[serviceName] = conn

	m.logger.DebugCtx(ctx, "✅ On-demand connection succeeded",
		zap.String("service", serviceName),
		zap.String("target", targetAddr),
		zap.Duration("timeout", timeout))

	return conn, nil
}

// handleInstancesUpdate 处理实例列表更新
// ✅ 简化策略：不主动重连，依赖 GetConn 时的 connectOnDemand 重试
// 原因：避免 Watch 触发频繁重连，造成连接抖动
func (m *ClientManager) handleInstancesUpdate(serviceName string, cfg ClientConfig, instances []*governance.ServiceInstance) {
	ctx := context.Background()

	m.logger.DebugCtx(ctx, "🔄 Service instance list updated",
		zap.String("service", serviceName),
		zap.Int("instances", len(instances)))

	// 🎯 策略：记录健康实例数量，不主动重连
	// 当前连接如果失败，下次 GetConn 会自动触发 connectOnDemand 重连到新实例

	healthyCount := 0
	for _, inst := range instances {
		if inst.Healthy {
			healthyCount++
		}
	}

	if healthyCount == 0 {
		m.logger.WarnCtx(ctx, "⚠️  No healthy instances currently, waiting for service recovery",
			zap.String("service", serviceName))
	} else {
		m.logger.InfoCtx(ctx, "✅ Healthy instances available",
			zap.String("service", serviceName),
			zap.Int("healthy_count", healthyCount))
	}

	// 可选优化：检测当前连接的实例是否已下线，提前断开连接
	// 这样下次 GetConn 会触发 connectOnDemand 重连到新实例
	// TODO: 实现连接健康检查（如果需要）
}

// reconnect 重新连接到新实例

// Close 关闭所有客户端连接
func (m *ClientManager) Close() {
	ctx := context.Background()

	// 停止所有Watch监听
	m.watchCancel()
	m.watchWg.Wait()

	m.mu.Lock()
	defer m.mu.Unlock()

	// 关闭所有gRPC连接
	for name, conn := range m.conns {
		if err := conn.Close(); err != nil {
			m.logger.ErrorCtx(ctx, "Failed to close gRPC connection",
				zap.String("conn", name),
				zap.Error(err),
			)
		} else {
			m.logger.DebugCtx(ctx, "🔌 Closing gRPC connection", zap.String("conn", name))
		}
	}
}
