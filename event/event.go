package event

import "time"

// Event 事件接口
type Event interface {
	// Name 事件名称（唯一标识，如 "admin.login"）
	Name() string
}

// BaseEvent 事件基类，可嵌入到具体事件结构体中
type BaseEvent struct {
	name       string
	occurredAt time.Time
}

// NewEvent 创建基础事件
func NewEvent(name string) BaseEvent {
	return BaseEvent{
		name:       name,
		occurredAt: time.Now(),
	}
}

// Name 返回事件名称
func (e BaseEvent) Name() string {
	return e.name
}

// OccurredAt 返回事件发生时间
func (e BaseEvent) OccurredAt() time.Time {
	return e.occurredAt
}

