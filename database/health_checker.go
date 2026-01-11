package database

import (
	"context"
	"fmt"
)

// HealthChecker 数据库健康检查器
type HealthChecker struct {
	manager *Manager
}

// NewHealthChecker 创建数据库健康检查器
func NewHealthChecker(manager *Manager) *HealthChecker {
	return &HealthChecker{
		manager: manager,
	}
}

// Name 检查项名称
func (h *HealthChecker) Name() string {
	return "database"
}

// Check 执行健康检查
func (h *HealthChecker) Check(ctx context.Context) error {
	if h.manager == nil {
		return fmt.Errorf("database manager not initialized")
	}

	// 检查所有数据库实例
	dbNames := h.manager.GetDBNames()
	if len(dbNames) == 0 {
		return fmt.Errorf("no database instances configured")
	}

	for _, name := range dbNames {
		db := h.manager.DB(name)
		if db == nil {
			return fmt.Errorf("database instance %s not found", name)
		}

		// Ping 数据库
		sqlDB, err := db.DB()
		if err != nil {
			return fmt.Errorf("failed to get sql.DB for %s: %w", name, err)
		}

		if err := sqlDB.PingContext(ctx); err != nil {
			return fmt.Errorf("database %s ping failed: %w", name, err)
		}
	}

	return nil
}
