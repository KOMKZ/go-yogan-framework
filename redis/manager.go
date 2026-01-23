package redis

import (
	"context"
	"fmt"
	"sync"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Manager for Redis (supports multiple instances, Cluster)
type Manager struct {
	instances map[string]*redis.Client        // single-machine instance
	clusters  map[string]*redis.ClusterClient // cluster instance
	configs   map[string]Config               // Configuration
	logger    *logger.CtxZapLogger            // Injector logger (supports TraceID)
	mu        sync.RWMutex                    // read-write lock
	metrics   *RedisMetrics                   // Optional: metrics provider (injected after creation)
}

// Create Redis manager
// configs: Redis configuration (supporting multiple instances)
// log: business logger (injected CtxZapLogger instance, must not be nil)
func NewManager(configs map[string]Config, log *logger.CtxZapLogger) (*Manager, error) {
	if log == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	ctx := context.Background()
	m := &Manager{
		instances: make(map[string]*redis.Client),
		clusters:  make(map[string]*redis.ClusterClient),
		configs:   make(map[string]Config),
		logger:    log,
	}

	// Initialize all instances
	for name, cfg := range configs {
		// Apply default values
		cfg.ApplyDefaults()

		// Validate configuration
		if err := cfg.Validate(); err != nil {
			return nil, fmt.Errorf("invalid config for %s: %w", name, err)
		}

		// Create client according to pattern
		if cfg.Mode == "standalone" {
			client, err := m.createClient(cfg)
			if err != nil {
				return nil, fmt.Errorf("failed to create client %s: %w", name, err)
			}
			m.instances[name] = client
		} else if cfg.Mode == "cluster" {
			cluster, err := m.createClusterClient(cfg)
			if err != nil {
				return nil, fmt.Errorf("failed to create cluster %s: %w", name, err)
			}
			m.clusters[name] = cluster
		}

		m.configs[name] = cfg

		m.logger.DebugCtx(ctx, "Redis connection successful",
			zap.String("name", name),
			zap.String("mode", cfg.Mode),
			zap.Strings("addrs", cfg.Addrs))
	}

	return m, nil
}

// createClient Create single-machine client
func (m *Manager) createClient(cfg Config) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Addrs[0], // Use the first address in single-machine mode
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		MaxRetries:   cfg.MaxRetries,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})

	// Test connection
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("ping failed: %w", err)
	}

	return client, nil
}

// createClusterClient Create cluster client
func (m *Manager) createClusterClient(cfg Config) (*redis.ClusterClient, error) {
	cluster := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:        cfg.Addrs, // Use all addresses in cluster mode
		Password:     cfg.Password,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		MaxRetries:   cfg.MaxRetries,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})

	// Test connection
	ctx := context.Background()
	if err := cluster.Ping(ctx).Err(); err != nil {
		cluster.Close()
		return nil, fmt.Errorf("ping cluster failed: %w", err)
	}

	return cluster, nil
}

// Client retrieves single-instance configuration
// name: instance name (key in configuration)
// Return nil if the instance does not exist or is not in single-machine mode
func (m *Manager) Client(name string) *redis.Client {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.instances[name]
}

// Get cluster instance
// name: instance name (key in configuration)
// Return nil if the instance does not exist or is not in cluster mode
func (m *Manager) Cluster(name string) *redis.ClusterClient {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.clusters[name]
}

// WithDB switch database (single machine mode only)
// name: instance name
// db: database number (0-15)
// Return a new Redis client connected to the specified database
// Return nil if the instance does not exist or is not in single-machine mode
//
// Usage:
//
// sessionRedis := manager.WithDB("main", 1) // use DB 1 for session storage
// cacheRedis := manager.WithDB("main", 2)   // Use DB 2 for caching
func (m *Manager) WithDB(name string, db int) *redis.Client {
	client := m.Client(name)
	if client == nil {
		return nil
	}

	// Copy configuration and modify DB
	opts := client.Options()
	opts.DB = db

	// Create new client
	newClient := redis.NewClient(opts)

	// Test connection
	ctx := context.Background()
	if err := newClient.Ping(ctx).Err(); err != nil {
		m.logger.ErrorCtx(ctx, "WithDB database connection failed",
			zap.String("name", name),
			zap.Int("db", db),
			zap.Error(err))
		newClient.Close()
		return nil
	}

	return newClient
}

// Ping check all connections
func (m *Manager) Ping(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check single-instance configuration
	for name, client := range m.instances {
		if err := client.Ping(ctx).Err(); err != nil {
			return fmt.Errorf("ping %s failed: %w", name, err)
		}
	}

	// Check cluster instance
	for name, cluster := range m.clusters {
		if err := cluster.Ping(ctx).Err(); err != nil {
			return fmt.Errorf("ping cluster %s failed: %w", name, err)
		}
	}

	return nil
}

// GetInstanceNames Retrieve all single-instance names
func (m *Manager) GetInstanceNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.instances))
	for name := range m.instances {
		names = append(names, name)
	}
	return names
}

// GetClusterNames Retrieve all cluster instance names
func (m *Manager) GetClusterNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.clusters))
	for name := range m.clusters {
		names = append(names, name)
	}
	return names
}

// Close all connections
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ctx := context.Background()

	// Shut down single-instance
	for name, client := range m.instances {
		if err := client.Close(); err != nil {
			m.logger.ErrorCtx(ctx, "failed to close Redis connection",
				zap.String("name", name),
				zap.Error(err))
		} else {
			m.logger.DebugCtx(ctx, "Redis connection closed",
				zap.String("name", name))
		}
	}

	// Shut down cluster instance
	for name, cluster := range m.clusters {
		if err := cluster.Close(); err != nil {
			m.logger.ErrorCtx(ctx, "failed to close Redis cluster connection",
				zap.String("name", name),
				zap.Error(err))
		} else {
			m.logger.DebugCtx(ctx, "Redis cluster connection closed",
				zap.String("name", name))
		}
	}

	return nil
}

// Shutdown implements the sambertx/ShutDownable interface
// For automatically closing Redis connections when the DI container shuts down
func (m *Manager) Shutdown() error {
	return m.Close()
}

// SetMetrics injects the RedisMetrics provider and adds Hooks to all clients
// This should be called after Manager creation, only when metrics are enabled
// Safe to call multiple times (idempotent)
func (m *Manager) SetMetrics(metrics *RedisMetrics) {
	if metrics == nil || !metrics.IsMetricsEnabled() {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Prevent duplicate injection
	if m.metrics != nil {
		return
	}

	m.metrics = metrics
	ctx := context.Background()

	// Add Hook to all standalone clients
	for name, client := range m.instances {
		hook := NewMetricsHook(metrics, name)
		client.AddHook(hook)

		// Register pool stats callback if enabled
		if metrics.config.RecordPoolStats {
			metrics.RegisterPoolCallback(name, func() PoolStats {
				stats := client.PoolStats()
				return PoolStats{
					ActiveCount: int64(stats.TotalConns - stats.IdleConns),
					IdleCount:   int64(stats.IdleConns),
				}
			})
		}

		m.logger.DebugCtx(ctx, "Redis Metrics Hook added",
			zap.String("instance", name),
			zap.String("mode", "standalone"))
	}

	// Add Hook to all cluster clients
	for name, cluster := range m.clusters {
		hook := NewMetricsHook(metrics, name)
		cluster.AddHook(hook)

		// Register pool stats callback if enabled
		if metrics.config.RecordPoolStats {
			metrics.RegisterPoolCallback(name, func() PoolStats {
				stats := cluster.PoolStats()
				return PoolStats{
					ActiveCount: int64(stats.TotalConns - stats.IdleConns),
					IdleCount:   int64(stats.IdleConns),
				}
			})
		}

		m.logger.DebugCtx(ctx, "Redis Metrics Hook added",
			zap.String("instance", name),
			zap.String("mode", "cluster"))
	}
}
