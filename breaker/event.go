package breaker

import (
	"context"
	"time"
)

// Event interface
type Event interface {
	Type() EventType
	Resource() string
	Timestamp() time.Time
	Context() context.Context
}

// Event Type
type EventType string

const (
	// EventStateChanged state change event
	EventStateChanged EventType = "state_changed"

	// EventCallSuccess Call success event
	EventCallSuccess EventType = "call_success"

	// EventCallFailure Call failure event
	EventCallFailure EventType = "call_failure"

	// EventCallTimeout call timeout event
	EventCallTimeout EventType = "call_timeout"

	// EventCallRejected Request rejected (circuit breaker active)
	EventCallRejected EventType = "call_rejected"

	// EventFallbackSuccess fallback success event
	EventFallbackSuccess EventType = "fallback_success"

	// EventFallbackFailure fallback failure event
	EventFallbackFailure EventType = "fallback_failure"

	// EventThresholdWarning approaching threshold warning
	EventThresholdWarning EventType = "threshold_warning"

	// EventThresholdExceeded Threshold Exceeded
	EventThresholdExceeded EventType = "threshold_exceeded"
)

// EventBus event bus interface
type EventBus interface {
	// Subscribe to events (with filter for event types)
	Subscribe(listener EventListener, filters ...EventType) SubscriptionID

	// Unsubscribe from subscription
	Unsubscribe(id SubscriptionID)

	// Publish internal event
	Publish(event Event)

	// Close event bus
	Close()
}

// EventListener (event listener implementation in the application layer)
type EventListener interface {
	OnEvent(event Event)
}

// Subscription ID
type SubscriptionID string

// Functional listener (convenient usage)
type EventListenerFunc func(event Event)

func (f EventListenerFunc) OnEvent(event Event) {
	f(event)
}

// BaseEvent basic event
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

// NewBaseEvent creates a base event
func NewBaseEvent(eventType EventType, resource string, ctx context.Context) BaseEvent {
	return BaseEvent{
		eventType: eventType,
		resource:  resource,
		timestamp: time.Now(),
		ctx:       ctx,
	}
}

// StateChangedEvent state change event
type StateChangedEvent struct {
	BaseEvent
	FromState State
	ToState   State
	Reason    string
	Metrics   *MetricsSnapshot
}

// CallEvent calls the event (success/failure/timeout)
type CallEvent struct {
	BaseEvent
	Success  bool
	Duration time.Duration
	Error    error
}

// RejectedEvent rejected event
type RejectedEvent struct {
	BaseEvent
	CurrentState State
}

// FallbackEvent fallback event
type FallbackEvent struct {
	BaseEvent
	Success  bool
	Duration time.Duration
	Error    error
}
