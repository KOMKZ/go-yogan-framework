package limiter

import (
	"context"
	"time"
)

// Event Type
type EventType string

const (
	// EventAllowed permission granted
	EventAllowed EventType = "allowed"

	// EventRejected reject request
	EventRejected EventType = "rejected"

	// EventWaitStart Start waiting
	EventWaitStart EventType = "wait_start"

	// EventWaitSuccess wait successful
	EventWaitSuccess EventType = "wait_success"

	// EventWaitTimeout wait timeout
	EventWaitTimeout EventType = "wait_timeout"

	// EventLimitChanged Threshold change for rate limiting (adaptive)
	EventLimitChanged EventType = "limit_changed"
)

// Event interface
type Event interface {
	Type() EventType
	Resource() string
	Context() context.Context
	Timestamp() time.Time
}

// BaseEvent basic event
type BaseEvent struct {
	eventType EventType
	resource  string
	ctx       context.Context
	timestamp time.Time
}

// NewBaseEvent creates a base event
func NewBaseEvent(eventType EventType, resource string, ctx context.Context) BaseEvent {
	return BaseEvent{
		eventType: eventType,
		resource:  resource,
		ctx:       ctx,
		timestamp: time.Now(),
	}
}

// Type Return event type
func (e *BaseEvent) Type() EventType {
	return e.eventType
}

// Resource returns resource
func (e *BaseEvent) Resource() string {
	return e.resource
}

// Context returns the context
func (e *BaseEvent) Context() context.Context {
	return e.ctx
}

// Return timestamp
func (e *BaseEvent) Timestamp() time.Time {
	return e.timestamp
}

// AllowedEvent permitted events
type AllowedEvent struct {
	BaseEvent
	Remaining int64
	Limit     int64
}

// RejectedEvent rejected event
type RejectedEvent struct {
	BaseEvent
	RetryAfter time.Duration
	Reason     string
}

// WaitEvent waiting event
type WaitEvent struct {
	BaseEvent
	Success bool
	Waited  time.Duration
}

// LimitChangedEvent Throttling threshold change event (adaptive)
type LimitChangedEvent struct {
	BaseEvent
	OldLimit    int64
	NewLimit    int64
	CPUUsage    float64
	MemoryUsage float64
	SystemLoad  float64
}

// EventListener event listener interface
type EventListener interface {
	OnEvent(event Event)
}

// EventListenerFunc event listener function type
type EventListenerFunc func(event Event)

// OnEvent implements EventListener interface
func (f EventListenerFunc) OnEvent(event Event) {
	f(event)
}

// EventBus event bus interface
type EventBus interface {
	// Subscribe to event
	Subscribe(listener EventListener)

	// Publish event
	Publish(event Event)

	// Close event bus
	Close()
}

