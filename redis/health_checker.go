package redis

import (
	"context"
	"fmt"
)

// HealthChecker for Redis
type HealthChecker struct {
	manager *Manager
}

// Create Redis health checker
func NewHealthChecker(manager *Manager) *HealthChecker {
	return &HealthChecker{
		manager: manager,
	}
}

// Name Check item name
func (h *HealthChecker) Name() string {
	return "redis"
}

// Check execution health check
func (h *HealthChecker) Check(ctx context.Context) error {
	if h.manager == nil {
		return fmt.Errorf("redis manager not initialized")
	}

	// Check all Redis instances
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

	// Check all cluster instances
	for _, name := range h.manager.GetClusterNames() {
		cluster := h.manager.Cluster(name)
		if cluster == nil {
			return fmt.Errorf("redis cluster %s not found", name)
		}

		// Ping cluster
		if err := cluster.Ping(ctx).Err(); err != nil {
			return fmt.Errorf("redis cluster %s ping failed: %w", name, err)
		}
	}

	return nil
}

