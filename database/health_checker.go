package database

import (
	"context"
	"fmt"
)

// HealthChecker database health checker
type HealthChecker struct {
	manager *Manager
}

// Create database health checker
func NewHealthChecker(manager *Manager) *HealthChecker {
	return &HealthChecker{
		manager: manager,
	}
}

// Name Check item name
func (h *HealthChecker) Name() string {
	return "database"
}

// Check execution health check
func (h *HealthChecker) Check(ctx context.Context) error {
	if h.manager == nil {
		return fmt.Errorf("database manager not initialized")
	}

	// Check all database instances
	dbNames := h.manager.GetDBNames()
	if len(dbNames) == 0 {
		return fmt.Errorf("no database instances configured")
	}

	for _, name := range dbNames {
		db := h.manager.DB(name)
		if db == nil {
			return fmt.Errorf("database instance %s not found", name)
		}

		// Ping database
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
