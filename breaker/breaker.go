// Package breaker 提供熔断器功能
// 
// 设计理念：
//   - 独立包，不依赖其他 yogan 组件（除 logger）
//   - 事件驱动，应用层可订阅所有事件
//   - 指标开放，应用层可访问和订阅实时数据
//   - 可选启用，未配置时不生效
package breaker

import (
	"context"
	"time"
)

// Breaker 熔断器核心接口
type Breaker interface {
	// Execute 执行受保护的调用
	Execute(ctx context.Context, req *Request) (*Response, error)
	
	// GetState 获取资源的当前状态
	GetState(resource string) State
	
	// GetMetrics 获取指标快照（应用层可访问）
	GetMetrics(resource string) *MetricsSnapshot
	
	// GetEventBus 获取事件总线（用于订阅事件）
	GetEventBus() EventBus
	
	// GetMetricsCollector 获取指标采集器（用于订阅实时数据）
	GetMetricsCollector(resource string) MetricsCollector
	
	// Reset 手动重置熔断器状态
	Reset(resource string)
	
	// Close 关闭熔断器（清理资源）
	Close() error
	
	// IsEnabled 检查熔断器是否启用（未配置时返回 false）
	IsEnabled() bool
}

// Request 请求上下文
type Request struct {
	// Resource 资源标识（服务名、方法名等）
	Resource string
	
	// Execute 实际调用函数
	Execute func(ctx context.Context) (interface{}, error)
	
	// Fallback 降级逻辑（可选）
	Fallback func(ctx context.Context, err error) (interface{}, error)
	
	// Timeout 超时时间（可选，0 表示使用配置的超时）
	Timeout time.Duration
}

// Response 响应结果
type Response struct {
	// Value 返回值
	Value interface{}
	
	// FromFallback 是否来自降级
	FromFallback bool
	
	// Duration 调用耗时
	Duration time.Duration
	
	// Error 错误（如果有）
	Error error
}

// State 熔断器状态
type State int

const (
	// StateClosed 关闭（正常）
	StateClosed State = iota
	
	// StateOpen 打开（熔断）
	StateOpen
	
	// StateHalfOpen 半开（试探恢复）
	StateHalfOpen
)

// String 返回状态名称
func (s State) String() string {
	switch s {
	case StateClosed:
		return "Closed"
	case StateOpen:
		return "Open"
	case StateHalfOpen:
		return "HalfOpen"
	default:
		return "Unknown"
	}
}

// IsOpen 是否处于熔断状态
func (s State) IsOpen() bool {
	return s == StateOpen
}

// IsClosed 是否处于正常状态
func (s State) IsClosed() bool {
	return s == StateClosed
}

// IsHalfOpen 是否处于半开状态
func (s State) IsHalfOpen() bool {
	return s == StateHalfOpen
}

