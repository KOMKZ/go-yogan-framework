package limiter

import (
	"context"
	"time"
)

// EventType 事件类型
type EventType string

const (
	// EventAllowed 允许通过
	EventAllowed EventType = "allowed"

	// EventRejected 拒绝请求
	EventRejected EventType = "rejected"

	// EventWaitStart 开始等待
	EventWaitStart EventType = "wait_start"

	// EventWaitSuccess 等待成功
	EventWaitSuccess EventType = "wait_success"

	// EventWaitTimeout 等待超时
	EventWaitTimeout EventType = "wait_timeout"

	// EventLimitChanged 限流阈值变化（自适应）
	EventLimitChanged EventType = "limit_changed"
)

// Event 事件接口
type Event interface {
	Type() EventType
	Resource() string
	Context() context.Context
	Timestamp() time.Time
}

// BaseEvent 基础事件
type BaseEvent struct {
	eventType EventType
	resource  string
	ctx       context.Context
	timestamp time.Time
}

// NewBaseEvent 创建基础事件
func NewBaseEvent(eventType EventType, resource string, ctx context.Context) BaseEvent {
	return BaseEvent{
		eventType: eventType,
		resource:  resource,
		ctx:       ctx,
		timestamp: time.Now(),
	}
}

// Type 返回事件类型
func (e *BaseEvent) Type() EventType {
	return e.eventType
}

// Resource 返回资源
func (e *BaseEvent) Resource() string {
	return e.resource
}

// Context 返回上下文
func (e *BaseEvent) Context() context.Context {
	return e.ctx
}

// Timestamp 返回时间戳
func (e *BaseEvent) Timestamp() time.Time {
	return e.timestamp
}

// AllowedEvent 允许事件
type AllowedEvent struct {
	BaseEvent
	Remaining int64
	Limit     int64
}

// RejectedEvent 拒绝事件
type RejectedEvent struct {
	BaseEvent
	RetryAfter time.Duration
	Reason     string
}

// WaitEvent 等待事件
type WaitEvent struct {
	BaseEvent
	Success bool
	Waited  time.Duration
}

// LimitChangedEvent 限流阈值变化事件（自适应）
type LimitChangedEvent struct {
	BaseEvent
	OldLimit    int64
	NewLimit    int64
	CPUUsage    float64
	MemoryUsage float64
	SystemLoad  float64
}

// EventListener 事件监听器接口
type EventListener interface {
	OnEvent(event Event)
}

// EventListenerFunc 事件监听器函数类型
type EventListenerFunc func(event Event)

// OnEvent 实现EventListener接口
func (f EventListenerFunc) OnEvent(event Event) {
	f(event)
}

// EventBus 事件总线接口
type EventBus interface {
	// Subscribe 订阅事件
	Subscribe(listener EventListener)

	// Publish 发布事件
	Publish(event Event)

	// Close 关闭事件总线
	Close()
}

