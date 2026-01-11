package breaker

import (
	"context"
	"time"
)

// Event 事件接口
type Event interface {
	Type() EventType
	Resource() string
	Timestamp() time.Time
	Context() context.Context
}

// EventType 事件类型
type EventType string

const (
	// EventStateChanged 状态变化事件
	EventStateChanged EventType = "state_changed"

	// EventCallSuccess 调用成功事件
	EventCallSuccess EventType = "call_success"

	// EventCallFailure 调用失败事件
	EventCallFailure EventType = "call_failure"

	// EventCallTimeout 调用超时事件
	EventCallTimeout EventType = "call_timeout"

	// EventCallRejected 请求被拒绝（熔断中）
	EventCallRejected EventType = "call_rejected"

	// EventFallbackSuccess 降级成功事件
	EventFallbackSuccess EventType = "fallback_success"

	// EventFallbackFailure 降级失败事件
	EventFallbackFailure EventType = "fallback_failure"

	// EventThresholdWarning 接近阈值告警
	EventThresholdWarning EventType = "threshold_warning"

	// EventThresholdExceeded 超过阈值
	EventThresholdExceeded EventType = "threshold_exceeded"
)

// EventBus 事件总线接口
type EventBus interface {
	// Subscribe 订阅事件（可过滤事件类型）
	Subscribe(listener EventListener, filters ...EventType) SubscriptionID

	// Unsubscribe 取消订阅
	Unsubscribe(id SubscriptionID)

	// Publish 发布事件（内部使用）
	Publish(event Event)

	// Close 关闭事件总线
	Close()
}

// EventListener 事件监听器（应用层实现）
type EventListener interface {
	OnEvent(event Event)
}

// SubscriptionID 订阅ID
type SubscriptionID string

// EventListenerFunc 函数式监听器（便捷用法）
type EventListenerFunc func(event Event)

func (f EventListenerFunc) OnEvent(event Event) {
	f(event)
}

// BaseEvent 基础事件
type BaseEvent struct {
	eventType EventType
	resource  string
	timestamp time.Time
	ctx       context.Context
}

func (e *BaseEvent) Type() EventType          { return e.eventType }
func (e *BaseEvent) Resource() string         { return e.resource }
func (e *BaseEvent) Timestamp() time.Time     { return e.timestamp }
func (e *BaseEvent) Context() context.Context { return e.ctx }

// NewBaseEvent 创建基础事件
func NewBaseEvent(eventType EventType, resource string, ctx context.Context) BaseEvent {
	return BaseEvent{
		eventType: eventType,
		resource:  resource,
		timestamp: time.Now(),
		ctx:       ctx,
	}
}

// StateChangedEvent 状态变化事件
type StateChangedEvent struct {
	BaseEvent
	FromState State
	ToState   State
	Reason    string
	Metrics   *MetricsSnapshot
}

// CallEvent 调用事件（成功/失败/超时）
type CallEvent struct {
	BaseEvent
	Success  bool
	Duration time.Duration
	Error    error
}

// RejectedEvent 拒绝事件
type RejectedEvent struct {
	BaseEvent
	CurrentState State
}

// FallbackEvent 降级事件
type FallbackEvent struct {
	BaseEvent
	Success  bool
	Duration time.Duration
	Error    error
}
