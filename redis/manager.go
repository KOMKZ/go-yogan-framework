package redis

import (
	"context"
	"fmt"
	"sync"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Manager Redis 管理器（支持多实例、Cluster）
type Manager struct {
	instances map[string]*redis.Client        // 单机实例
	clusters  map[string]*redis.ClusterClient // 集群实例
	configs   map[string]Config               // 配置
	logger    *zap.Logger                     // 注入的日志器
	mu        sync.RWMutex                    // 读写锁
}

// NewManager 创建 Redis 管理器
// configs: Redis 配置（支持多实例）
// logger: 业务日志器（注入的 zap.Logger 实例，不能为 nil）
func NewManager(configs map[string]Config, logger *zap.Logger) (*Manager, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	m := &Manager{
		instances: make(map[string]*redis.Client),
		clusters:  make(map[string]*redis.ClusterClient),
		configs:   make(map[string]Config),
		logger:    logger,
	}

	// 初始化所有实例
	for name, cfg := range configs {
		// 应用默认值
		cfg.ApplyDefaults()

		// 验证配置
		if err := cfg.Validate(); err != nil {
			return nil, fmt.Errorf("invalid config for %s: %w", name, err)
		}

		// 根据模式创建客户端
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

		m.logger.Debug("Redis 连接成功",
			zap.String("name", name),
			zap.String("mode", cfg.Mode),
			zap.Strings("addrs", cfg.Addrs))
	}

	return m, nil
}

// createClient 创建单机客户端
func (m *Manager) createClient(cfg Config) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Addrs[0], // 单机模式使用第一个地址
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		MaxRetries:   cfg.MaxRetries,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})

	// 测试连接
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("ping failed: %w", err)
	}

	return client, nil
}

// createClusterClient 创建集群客户端
func (m *Manager) createClusterClient(cfg Config) (*redis.ClusterClient, error) {
	cluster := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:        cfg.Addrs, // 集群模式使用所有地址
		Password:     cfg.Password,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		MaxRetries:   cfg.MaxRetries,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})

	// 测试连接
	ctx := context.Background()
	if err := cluster.Ping(ctx).Err(); err != nil {
		cluster.Close()
		return nil, fmt.Errorf("ping cluster failed: %w", err)
	}

	return cluster, nil
}

// Client 获取单机实例
// name: 实例名称（配置中的 key）
// 返回 nil 如果实例不存在或不是单机模式
func (m *Manager) Client(name string) *redis.Client {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.instances[name]
}

// Cluster 获取集群实例
// name: 实例名称（配置中的 key）
// 返回 nil 如果实例不存在或不是集群模式
func (m *Manager) Cluster(name string) *redis.ClusterClient {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.clusters[name]
}

// WithDB 切换数据库（仅单机模式）
// name: 实例名称
// db: 数据库编号（0-15）
// 返回一个新的 Redis 客户端，连接到指定的数据库
// 返回 nil 如果实例不存在或不是单机模式
//
// 用法：
//
//	sessionRedis := manager.WithDB("main", 1) // 使用 DB 1 作为 Session 存储
//	cacheRedis := manager.WithDB("main", 2)   // 使用 DB 2 作为缓存
func (m *Manager) WithDB(name string, db int) *redis.Client {
	client := m.Client(name)
	if client == nil {
		return nil
	}

	// 复制配置并修改 DB
	opts := client.Options()
	opts.DB = db

	// 创建新的客户端
	newClient := redis.NewClient(opts)

	// 测试连接
	ctx := context.Background()
	if err := newClient.Ping(ctx).Err(); err != nil {
		m.logger.Error("WithDB 连接失败",
			zap.String("name", name),
			zap.Int("db", db),
			zap.Error(err))
		newClient.Close()
		return nil
	}

	return newClient
}

// Ping 检查所有连接
func (m *Manager) Ping(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 检查单机实例
	for name, client := range m.instances {
		if err := client.Ping(ctx).Err(); err != nil {
			return fmt.Errorf("ping %s failed: %w", name, err)
		}
	}

	// 检查集群实例
	for name, cluster := range m.clusters {
		if err := cluster.Ping(ctx).Err(); err != nil {
			return fmt.Errorf("ping cluster %s failed: %w", name, err)
		}
	}

	return nil
}

// GetInstanceNames 获取所有单机实例名称
func (m *Manager) GetInstanceNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.instances))
	for name := range m.instances {
		names = append(names, name)
	}
	return names
}

// GetClusterNames 获取所有集群实例名称
func (m *Manager) GetClusterNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.clusters))
	for name := range m.clusters {
		names = append(names, name)
	}
	return names
}

// Close 关闭所有连接
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 关闭单机实例
	for name, client := range m.instances {
		if err := client.Close(); err != nil {
			m.logger.Error("关闭 Redis 连接失败",
				zap.String("name", name),
				zap.Error(err))
		} else {
			m.logger.Debug("Redis 连接已关闭",
				zap.String("name", name))
		}
	}

	// 关闭集群实例
	for name, cluster := range m.clusters {
		if err := cluster.Close(); err != nil {
			m.logger.Error("关闭 Redis 集群连接失败",
				zap.String("name", name),
				zap.Error(err))
		} else {
			m.logger.Debug("Redis 集群连接已关闭",
				zap.String("name", name))
		}
	}

	return nil
}

// Shutdown 实现 samber/do.Shutdownable 接口
// 用于在 DI 容器关闭时自动关闭 Redis 连接
func (m *Manager) Shutdown() error {
	return m.Close()
}
