package redis

import (
	"context"
	"fmt"
)

// HealthChecker Redis 健康检查器
type HealthChecker struct {
	manager *Manager
}

// NewHealthChecker 创建 Redis 健康检查器
func NewHealthChecker(manager *Manager) *HealthChecker {
	return &HealthChecker{
		manager: manager,
	}
}

// Name 检查项名称
func (h *HealthChecker) Name() string {
	return "redis"
}

// Check 执行健康检查
func (h *HealthChecker) Check(ctx context.Context) error {
	if h.manager == nil {
		return fmt.Errorf("redis manager not initialized")
	}

	// 检查所有 Redis 实例
	for _, name := range h.manager.GetInstanceNames() {
		client := h.manager.Client(name)
		if client == nil {
			return fmt.Errorf("redis instance %s not found", name)
		}

		// Ping Redis
		if err := client.Ping(ctx).Err(); err != nil {
			return fmt.Errorf("redis %s ping failed: %w", name, err)
		}
	}

	// 检查所有集群实例
	for _, name := range h.manager.GetClusterNames() {
		cluster := h.manager.Cluster(name)
		if cluster == nil {
			return fmt.Errorf("redis cluster %s not found", name)
		}

		// Ping 集群
		if err := cluster.Ping(ctx).Err(); err != nil {
			return fmt.Errorf("redis cluster %s ping failed: %w", name, err)
		}
	}

	return nil
}

