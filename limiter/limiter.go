// Package limiter 提供限流器功能
//
// 设计理念：
//   - 独立包，不依赖其他 yogan 组件（除 logger）
//   - 事件驱动，应用层可订阅所有事件
//   - 指标开放，应用层可访问实时数据
//   - 可选启用，未配置时不生效
//   - 支持多种算法：令牌桶、滑动窗口、并发限流、自适应
//   - 支持多种存储：内存、Redis
package limiter

import (
	"context"
	"time"
)

// Limiter 限流器核心接口
type Limiter interface {
	// Allow 检查是否允许请求（快速检查）
	Allow(ctx context.Context, resource string) (bool, error)

	// AllowN 检查是否允许N个请求
	AllowN(ctx context.Context, resource string, n int64) (bool, error)

	// Wait 等待获取许可（阻塞等待，支持超时）
	Wait(ctx context.Context, resource string) error

	// WaitN 等待获取N个许可
	WaitN(ctx context.Context, resource string, n int64) error

	// GetMetrics 获取指标快照
	GetMetrics(resource string) *MetricsSnapshot

	// GetEventBus 获取事件总线（用于订阅事件）
	GetEventBus() EventBus

	// Reset 重置限流器状态
	Reset(resource string)

	// Close 关闭限流器（清理资源）
	Close() error

	// IsEnabled 检查限流器是否启用
	IsEnabled() bool
}

// Response 限流响应
type Response struct {
	// Allowed 是否允许
	Allowed bool

	// RetryAfter 建议重试时间（Allowed=false时有效）
	RetryAfter time.Duration

	// Remaining 剩余配额（令牌桶/滑动窗口）
	Remaining int64

	// Limit 总限额
	Limit int64

	// ResetAt 配额重置时间
	ResetAt time.Time
}

