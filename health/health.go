// Package health 提供统一的健康检查能力
package health

import (
	"time"

	"github.com/KOMKZ/go-yogan-framework/component"
)

// Status 健康状态枚举
type Status string

const (
	// StatusHealthy 健康
	StatusHealthy Status = "healthy"
	// StatusDegraded 降级（部分功能不可用）
	StatusDegraded Status = "degraded"
	// StatusUnhealthy 不健康
	StatusUnhealthy Status = "unhealthy"
)

// Checker 是 component.HealthChecker 的别名，方便使用
type Checker = component.HealthChecker

// CheckResult 单个检查项的结果
type CheckResult struct {
	Name      string        `json:"name"`               // 检查项名称
	Status    Status        `json:"status"`             // 健康状态
	Message   string        `json:"message,omitempty"`  // 状态消息
	Error     string        `json:"error,omitempty"`    // 错误信息
	Timestamp time.Time     `json:"timestamp"`          // 检查时间
	Duration  time.Duration `json:"duration,omitempty"` // 检查耗时
}

// Response 健康检查响应
type Response struct {
	Status    Status                 `json:"status"`             // 整体健康状态
	Timestamp time.Time              `json:"timestamp"`          // 检查时间
	Duration  time.Duration          `json:"duration"`           // 总检查耗时
	Checks    map[string]CheckResult `json:"checks"`             // 各检查项结果
	Metadata  map[string]interface{} `json:"metadata,omitempty"` // 元数据
}

// IsHealthy 判断整体是否健康
func (r *Response) IsHealthy() bool {
	return r.Status == StatusHealthy
}

// IsDegraded 判断是否降级
func (r *Response) IsDegraded() bool {
	return r.Status == StatusDegraded
}
