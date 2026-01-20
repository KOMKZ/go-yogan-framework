package event

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"time"
)

// KafkaEventPayload Kafka event message format
type KafkaEventPayload struct {
	EventName  string          `json:"event_name"`
	Payload    json.RawMessage `json:"payload"`
	OccurredAt time.Time       `json:"occurred_at"`
	TraceID    string          `json:"trace_id,omitempty"`
}

// SerializeEvent serialize event to KafkaEventPayload
func SerializeEvent(event Event, traceID string) (*KafkaEventPayload, error) {
	payload, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("marshal event failed: %w", err)
	}

	return &KafkaEventPayload{
		EventName:  string(event.Name()),
		Payload:    payload,
		OccurredAt: time.Now(),
		TraceID:    traceID,
	}, nil
}

// eventRegistry event registry (for deserialization)
var (
	eventRegistry   = make(map[string]reflect.Type)
	eventRegistryMu sync.RWMutex
)

// RegisterEventType Register event type (for deserialization)
// Called at application startup to register all event types that need to consume from Kafka
// T should be a pointer type, such as RegisterEventType<UserCreatedEvent\*>()
func RegisterEventType[T Event]() {
	typ := reflect.TypeOf((*T)(nil)).Elem()

	// If it is a pointer type, get the element type
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	// Create a zero-valued instance to obtain the event name
	instance := reflect.New(typ).Interface().(Event)
	name := instance.Name()

	eventRegistryMu.Lock()
	defer eventRegistryMu.Unlock()
	eventRegistry[name] = typ
}

// DeserializeEvent deserialize event
func DeserializeEvent(payload *KafkaEventPayload) (Event, error) {
	eventRegistryMu.RLock()
	typ, ok := eventRegistry[payload.EventName]
	eventRegistryMu.RUnlock()

	if !ok {
		// Unregistered event type, return GenericEvent
		return &GenericEvent{
			name:    payload.EventName,
			payload: payload.Payload,
		}, nil
	}

	// Create event instance
	eventPtr := reflect.New(typ).Interface()
	if err := json.Unmarshal(payload.Payload, eventPtr); err != nil {
		return nil, fmt.Errorf("unmarshal event %s failed: %w", payload.EventName, err)
	}

	return eventPtr.(Event), nil
}

// GenericEvent generic event (for unregistered types)
type GenericEvent struct {
	name    string
	payload json.RawMessage
}

// Return event name
func (e *GenericEvent) Name() string {
	return e.name
}

// Payload returns original payload
func (e *GenericEvent) Payload() json.RawMessage {
	return e.payload
}

// GetRegisteredEventNames Get the list of registered event names
func GetRegisteredEventNames() []string {
	eventRegistryMu.RLock()
	defer eventRegistryMu.RUnlock()

	names := make([]string, 0, len(eventRegistry))
	for name := range eventRegistry {
		names = append(names, name)
	}
	return names
}
