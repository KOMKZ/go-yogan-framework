package kafka

import (
	"context"
	"fmt"
	"time"
)

// HealthChecker Kafka 健康检查器
type HealthChecker struct {
	manager *Manager
	timeout time.Duration
}

// NewHealthChecker 创建健康检查器
func NewHealthChecker(manager *Manager) *HealthChecker {
	return &HealthChecker{
		manager: manager,
		timeout: 5 * time.Second,
	}
}

// Name 返回检查项名称
func (h *HealthChecker) Name() string {
	return "kafka"
}

// Check 执行健康检查
func (h *HealthChecker) Check(ctx context.Context) error {
	if h.manager == nil {
		return fmt.Errorf("kafka manager is nil")
	}

	// 创建带超时的 context
	checkCtx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()

	return h.manager.Ping(checkCtx)
}

// SetTimeout 设置超时时间
func (h *HealthChecker) SetTimeout(timeout time.Duration) {
	h.timeout = timeout
}

