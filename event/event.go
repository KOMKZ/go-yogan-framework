package event

import "time"

// Event event interface
type Event interface {
	// Name event name (unique identifier, such as "admin.login")
	Name() string
}

// BaseEvent base class for events, can be embedded into specific event structs
type BaseEvent struct {
	name       string
	occurredAt time.Time
}

// Create base event
func NewEvent(name string) BaseEvent {
	return BaseEvent{
		name:       name,
		occurredAt: time.Now(),
	}
}

// Returns the event name
func (e BaseEvent) Name() string {
	return e.name
}

// OccurredAt returns the event occurrence time
func (e BaseEvent) OccurredAt() time.Time {
	return e.occurredAt
}

