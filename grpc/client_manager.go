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

// ClientManager gRPC client connection pool manager (supports service discovery)
type ClientManager struct {
	configs        map[string]ClientConfig
	conns          map[string]*grpc.ClientConn
	timeouts       map[string]time.Duration // timeout configuration for each client
	mu             sync.RWMutex
	logger         *logger.CtxZapLogger
	discovery      *governance.EtcdDiscovery // Service Discoverer (optional)
	selector       InstanceSelector          // Instance selector (optional, default FirstHealthy)
	breaker        *breaker.Manager          // circuit breaker (optional)
	limiter        *limiter.Manager          // 🎯 Speed Limit Manager (optional)
	tracerProvider trace.TracerProvider      // 🎯 OpenTelemetry TracerProvider (optional)
	// Watch related
	watchCtx    context.Context
	watchCancel context.CancelFunc
	watchWg     sync.WaitGroup
}

// Create client manager
func NewClientManager(configs map[string]ClientConfig, log *logger.CtxZapLogger) *ClientManager {
	ctx, cancel := context.WithCancel(context.Background())

	// Precompute the timeout for each client
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

// SetDiscovery set service discoverer (component layer injection)
func (m *ClientManager) SetDiscovery(discovery *governance.EtcdDiscovery) {
	m.discovery = discovery
}

// SetSelector Sets the instance selector (optional, defaults to FirstHealthy)
func (m *ClientManager) SetSelector(selector InstanceSelector) {
	m.selector = selector
}

// SetBreaker sets the circuit breaker (injected by the gRPC component at Start time)
func (m *ClientManager) SetBreaker(b *breaker.Manager) {
	m.breaker = b
	ctx := context.Background()
	if b != nil {
		m.logger.DebugCtx(ctx, "✅ Circuit breaker injected into gRPC client manager")
	}
}

// GetBreaker get circuit breaker status
func (m *ClientManager) GetBreaker() *breaker.Manager {
	return m.breaker
}

// SetLimiter sets the rate limiter manager (injected by the gRPC component at Start)
func (m *ClientManager) SetLimiter(lim *limiter.Manager) {
	m.limiter = lim
	ctx := context.Background()
	if lim != nil && lim.IsEnabled() {
		m.logger.DebugCtx(ctx, "✅ Rate limiter injected into gRPC client manager")
	}
}

// SetTracerProvider set TracerProvider
func (m *ClientManager) SetTracerProvider(tp trace.TracerProvider) {
	m.tracerProvider = tp
	ctx := context.Background()
	if tp != nil {
		m.logger.DebugCtx(ctx, "✅ TracerProvider injected into gRPC client manager")
	}
}

// SetMetricsHandler sets the Metrics StatsHandler
// Note: The current implementation uses it when creating a connection, needs to be called before PreConnect
func (m *ClientManager) SetMetricsHandler(handler interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Temporarily not storing the handler, as the client's metrics are integrated through otelgrpc.NewClientHandler
	// This is only for interface compatibility
	ctx := context.Background()
	m.logger.DebugCtx(ctx, "✅ Metrics StatsHandler set in ClientManager (placeholder)")
}

// GetLimiter obtain speed limit manager
func (m *ClientManager) GetLimiter() *limiter.Manager {
	return m.limiter
}

// getSelector Get selector (with default values)
func (m *ClientManager) getSelector() InstanceSelector {
	if m.selector == nil {
		return NewFirstHealthySelector() // Default policy
	}
	return m.selector
}

// PreConnect asynchronously pre-connects all configured clients (supports service discovery and direct connection)
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

			// 🎯 Select connection mode based on configuration
			if config.DiscoveryMode != "" && config.ServiceName != "" {
				m.preConnectWithDiscovery(name, config, timeout)
			} else {
				m.preConnectDirect(name, config, timeout)
			}
		}(serviceName, cfg)
	}

	// wait for all connections to complete (or timeout)
	wg.Wait()
	m.logger.DebugCtx(ctx, "🔗 Pre-connection completed",
		zap.Int("conns", len(m.conns)),
		zap.Int("total", len(m.configs)))
}

// ========================================
// Public method: Eliminate duplicate code (DRY principle)
// ========================================

// discover and select healthy instance
// Return: instance address, error message
func (m *ClientManager) discoverHealthyInstance(ctx context.Context, serviceName string) (string, error) {
	if m.discovery == nil {
		return "", fmt.Errorf("Service discovery not initialized")
	}

	instances, err := m.discovery.Discover(ctx, serviceName)
	if err != nil {
		return "", fmt.Errorf("Service discovery query failed: %w", err)
	}

	if len(instances) == 0 {
		return "", fmt.Errorf("Service instance not found: %s", serviceName)
	}

	// Use the injected selector to choose an instance
	selected := m.getSelector().Select(instances)
	if selected == nil {
		return "", fmt.Errorf("No healthy service instances: %s", serviceName)
	}

	return selected.GetAddress(), nil
}

// dialWithOptions establishes a gRPC connection (reuses dialing logic)
func (m *ClientManager) dialWithOptions(ctx context.Context, serviceName, targetAddr string, cfg ClientConfig) (*grpc.ClientConn, error) {
	// Create a logger specific for client interceptors
	clientLogger := logger.GetLogger("yogan")
	enableLog := cfg.IsLogEnabled()

	// Get timeout configuration
	timeout := m.timeouts[serviceName]

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(), // block waiting for connection success
	}

	// 🎯 1. Add StatsHandler (highest priority, for OpenTelemetry)
	if m.tracerProvider != nil {
		opts = append(opts, grpc.WithStatsHandler(
			otelgrpc.NewClientHandler(
				otelgrpc.WithTracerProvider(m.tracerProvider),
			),
		))
	}

	// 🎯 2. Build interceptor chain (excluding OTel, already handled by StatsHandler)
	interceptors := []grpc.UnaryClientInterceptor{
		UnaryClientTraceInterceptor(),                         // 1️⃣ Propagate TraceID
		UnaryClientRateLimitInterceptor(m, serviceName),       // Speed limit check
		UnaryClientBreakerInterceptor(m, serviceName),         // 3️⃣ Circuit breaker
		UnaryClientTimeoutInterceptor(timeout, clientLogger),  // 4️⃣ Timeout control
		UnaryClientLoggerInterceptor(clientLogger, enableLog), // 5️⃣ Logging (configurable)
	}
	opts = append(opts, grpc.WithChainUnaryInterceptor(interceptors...))

	// 3. Service discovery pattern adds load balancing configuration
	if cfg.LoadBalance != "" {
		opts = append(opts, grpc.WithDefaultServiceConfig(
			fmt.Sprintf(`{"loadBalancingPolicy":"%s"}`, cfg.LoadBalance)))
	}

	return grpc.DialContext(ctx, targetAddr, opts...)
}

// preConnectWithDiscovery service discovery mode pre-connection
// ✅ After refactoring: Watch starts independently and pre-connects as best effort
func (m *ClientManager) preConnectWithDiscovery(serviceName string, cfg ClientConfig, timeout time.Duration) {
	// Step 1: Unconditionally start Watch listening (independent lifecycle)
	m.startWatchForever(serviceName, cfg)

	// ✅ Step 2: Attempt a pre-connection (best effort, failure does not impact Watch)
	m.tryPreConnect(serviceName, cfg, timeout)
}

// tryPreConnect attempts to establish a pre-connection (single responsibility: connection establishment)
func (m *ClientManager) tryPreConnect(serviceName string, cfg ClientConfig, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 1. Discover healthy instances
	targetAddr, err := m.discoverHealthyInstance(ctx, cfg.ServiceName)
	if err != nil {
		m.logger.WarnCtx(ctx, "⚠️  Pre-connection failed (service discovery), will auto-retry at runtime",
			zap.String("service", serviceName),
			zap.String("target_service", cfg.ServiceName),
			zap.Error(err))
		return
	}

	// Establish connection
	conn, err := m.dialWithOptions(ctx, serviceName, targetAddr, cfg)
	if err != nil {
		m.logger.WarnCtx(ctx, "⚠️  Pre-connection failed (connection establishment), will auto-retry at runtime",
			zap.String("service", serviceName),
			zap.String("target", targetAddr),
			zap.Error(err))
		return
	}

	// 3. Cache connection
	m.mu.Lock()
	m.conns[serviceName] = conn
	m.mu.Unlock()

	m.logger.DebugCtx(ctx, "✅ Pre-connection succeeded (service discovery mode)",
		zap.String("service", serviceName),
		zap.String("target_service", cfg.ServiceName),
		zap.String("target", targetAddr),
		zap.String("load_balance", cfg.LoadBalance))
}

// startWatchForever starts watch listening (never gives up, auto retries)
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
				// Try to start the Watch loop
				err := m.runWatchLoop(serviceName, cfg)
				if err != nil {
					m.logger.WarnCtx(context.Background(),
						"⚠️  Watch interrupted, will retry later",
						zap.String("service", serviceName),
						zap.String("target_service", cfg.ServiceName),
						zap.Error(err),
						zap.Duration("retry_after", backoff))

					// Exponential backoff retry
					select {
					case <-m.watchCtx.Done():
						return
					case <-time.After(backoff):
						backoff = min(backoff*2, maxBackoff)
					}
				} else {
					// Normal exit, reset backoff
					backoff = time.Second
				}
			}
		}
	}()
}

// runWatchLoop runs one Watch loop (single responsibility)
func (m *ClientManager) runWatchLoop(serviceName string, cfg ClientConfig) error {
	ctx := context.Background()

	watchCh, err := m.discovery.Watch(ctx, cfg.ServiceName)
	if err != nil {
		return fmt.Errorf("Failed to start Watch: %w", err)
	}

	m.logger.DebugCtx(ctx, "🔍 Service instance watch started",
		zap.String("service", serviceName),
		zap.String("target_service", cfg.ServiceName))

	for {
		select {
		case <-m.watchCtx.Done():
			return nil // normal exit

		case instances, ok := <-watchCh:
			if !ok {
				return fmt.Errorf("WatchWatch channel closed")
			}

			// Handle instance update
			m.handleInstancesUpdate(serviceName, cfg, instances)
		}
	}
}

// preConnectDirect Direct connection mode pre-connection
func (m *ClientManager) preConnectDirect(serviceName string, cfg ClientConfig, timeout time.Duration) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Use dialWithOptions to uniformly create connections
	conn, err := m.dialWithOptions(ctx, serviceName, cfg.Target, cfg)
	if err != nil {
		m.logger.ErrorCtx(ctx, "❌ Pre-connection failed (service may be unavailable, will retry at runtime)",
			zap.String("service", serviceName),
			zap.String("target", cfg.Target),
			zap.Error(err),
			zap.Stack("stack"))
		return
	}

	// cache connection
	m.mu.Lock()
	m.conns[serviceName] = conn
	m.mu.Unlock()

	m.logger.DebugCtx(ctx, "✅ Pre-connection succeeded (direct mode)",
		zap.String("service", serviceName),
		zap.String("target", cfg.Target))
}

// GetConn obtain client connection (runtime call)
func (m *ClientManager) GetConn(serviceName string) (*grpc.ClientConn, error) {
	// Check if configuration exists
	cfg, ok := m.configs[serviceName]
	if !ok {
		return nil, fmt.Errorf("Service not configured: %s", serviceName)
	}

	m.mu.RLock()
	conn, exists := m.conns[serviceName]
	m.mu.RUnlock()

	if exists {
		return conn, nil
	}

	// 🎯 Runtime dynamic linking (if pre-linking fails)
	return m.connectOnDemand(serviceName, cfg)
}

// connectOnDemand Connect on demand (runtime retry)
// ✅ Refactored: Reuse common logic
func (m *ClientManager) connectOnDemand(serviceName string, cfg ClientConfig) (*grpc.ClientConn, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// double check
	if conn, exists := m.conns[serviceName]; exists {
		return conn, nil
	}

	// Use the configured timeout period
	timeout := time.Duration(cfg.GetTimeout()) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var targetAddr string
	var err error

	// 🎯 Service discovery pattern: Reuse discoverHealthyInstance
	if cfg.DiscoveryMode != "" && cfg.ServiceName != "" && m.discovery != nil {
		targetAddr, err = m.discoverHealthyInstance(ctx, cfg.ServiceName)
		if err != nil {
			return nil, fmt.Errorf("Service discovery failed: %w", err)
		}
	} else {
		// Direct connection mode
		targetAddr = cfg.Target
	}

	// ✅ Reuse dialWithOptions to establish connection
	conn, err := m.dialWithOptions(ctx, serviceName, targetAddr, cfg)
	if err != nil {
		return nil, fmt.Errorf("Connection failed: %w", err)
	}

	// Cache connection
	m.conns[serviceName] = conn

	m.logger.DebugCtx(ctx, "✅ On-demand connection succeeded",
		zap.String("service", serviceName),
		zap.String("target", targetAddr),
		zap.Duration("timeout", timeout))

	return conn, nil
}

// handleInstancesUpdate Handle instance list update
// ✅ Simplified strategy: do not proactively reconnect; rely on connectOnDemand retries when getting a connection
// Reason: To avoid frequent reconnection triggered by Watch, causing connection jitter
func (m *ClientManager) handleInstancesUpdate(serviceName string, cfg ClientConfig, instances []*governance.ServiceInstance) {
	ctx := context.Background()

	m.logger.DebugCtx(ctx, "🔄 Service instance list updated",
		zap.String("service", serviceName),
		zap.Int("instances", len(instances)))

	// 🎯 Strategy: Record the number of healthy instances, do not proactively reconnect
	// If the current connection fails, the next GetConn will automatically trigger a connectOnDemand to reconnect to a new instance.

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

	// Optional optimization: detect if currently connected instances are offline and disconnect in advance
	// This way, the next GetConn will trigger a connectOnDemand reconnection to the new instance
	// TODO: Implement connection health check (if necessary)
}

// reconnect to new instance

// Close all client connections
func (m *ClientManager) Close() {
	ctx := context.Background()

	// Stop all Watch listeners
	m.watchCancel()
	m.watchWg.Wait()

	m.mu.Lock()
	defer m.mu.Unlock()

	// Close all gRPC connections
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
