package kafka

import (
	"context"
	"fmt"
	"time"
)

// HealthChecker for Kafka
type HealthChecker struct {
	manager *Manager
	timeout time.Duration
}

// Create health checker
func NewHealthChecker(manager *Manager) *HealthChecker {
	return &HealthChecker{
		manager: manager,
		timeout: 5 * time.Second,
	}
}

// Name Returns the inspection item name
func (h *HealthChecker) Name() string {
	return "kafka"
}

// Check execution health check
func (h *HealthChecker) Check(ctx context.Context) error {
	if h.manager == nil {
		return fmt.Errorf("kafka manager is nil")
	}

	// Create a context with timeout
	checkCtx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()

	return h.manager.Ping(checkCtx)
}

// Set timeout duration
func (h *HealthChecker) SetTimeout(timeout time.Duration) {
	h.timeout = timeout
}

