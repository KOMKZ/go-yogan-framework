package governance

import (
	"context"
)

// HealthChecker 健康检查接口
type HealthChecker interface {
	// Check 执行健康检查
	// 返回 nil 表示健康，返回 error 表示不健康
	Check(ctx context.Context) error

	// GetStatus 获取健康状态
	GetStatus() HealthStatus
}

// HealthStatus 健康状态
type HealthStatus struct {
	Healthy bool              `json:"healthy"` // 是否健康
	Message string            `json:"message"` // 状态消息
	Details map[string]string `json:"details"` // 详细信息
}

// DefaultHealthChecker 默认健康检查器（始终返回健康）
type DefaultHealthChecker struct{}

// NewDefaultHealthChecker 创建默认健康检查器
func NewDefaultHealthChecker() *DefaultHealthChecker {
	return &DefaultHealthChecker{}
}

// Check 执行健康检查（默认实现：始终健康）
func (h *DefaultHealthChecker) Check(ctx context.Context) error {
	return nil
}

// GetStatus 获取健康状态
func (h *DefaultHealthChecker) GetStatus() HealthStatus {
	return HealthStatus{
		Healthy: true,
		Message: "OK",
	}
}

