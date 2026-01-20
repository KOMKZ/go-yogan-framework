package event

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// serializableEvent test using serializable event
// Use public fields for easy JSON serialization/deserialization
type serializableEvent struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
}

// Name returns a fixed event name
func (e *serializableEvent) Name() string {
	return "user.created"
}

func newSerializableEvent(userID int, username string) *serializableEvent {
	return &serializableEvent{
		UserID:   userID,
		Username: username,
	}
}

// TestSerializeEvent test event serialization
func TestSerializeEvent(t *testing.T) {
	event := newSerializableEvent(123, "john")
	payload, err := SerializeEvent(event, "trace-123")

	require.NoError(t, err)
	assert.Equal(t, "user.created", payload.EventName)
	assert.Equal(t, "trace-123", payload.TraceID)
	assert.NotZero(t, payload.OccurredAt)

	// Validate Payload content
	var data map[string]any
	err = json.Unmarshal(payload.Payload, &data)
	require.NoError(t, err)
	assert.Equal(t, float64(123), data["user_id"])
	assert.Equal(t, "john", data["username"])
}

// TestSerializeEvent_NoTraceID test without TraceID
func TestSerializeEvent_NoTraceID(t *testing.T) {
	event := newSerializableEvent(456, "jane")
	payload, err := SerializeEvent(event, "")

	require.NoError(t, err)
	assert.Empty(t, payload.TraceID)
}

// TestDeserializeEvent_Registered test deserialization of registered events
func TestDeserializeEvent_Registered(t *testing.T) {
	// Register event type (using pointer type)
	RegisterEventType[*serializableEvent]()

	// Create payload
	original := newSerializableEvent(789, "alice")
	payload, err := SerializeEvent(original, "trace-456")
	require.NoError(t, err)

	// deserialize
	event, err := DeserializeEvent(payload)
	require.NoError(t, err)

	// Validate type
	userEvent, ok := event.(*serializableEvent)
	require.True(t, ok)
	assert.Equal(t, 789, userEvent.UserID)
	assert.Equal(t, "alice", userEvent.Username)
}

// TestDeserializeEvent_Unregistered test deserialization of unregistered event
func TestDeserializeEvent_Unregistered(t *testing.T) {
	payload := &KafkaEventPayload{
		EventName: "unknown.event",
		Payload:   json.RawMessage(`{"foo":"bar"}`),
	}

	event, err := DeserializeEvent(payload)
	require.NoError(t, err)

	// Should return GenericEvent
	genericEvent, ok := event.(*GenericEvent)
	require.True(t, ok)
	assert.Equal(t, "unknown.event", genericEvent.Name())
	assert.JSONEq(t, `{"foo":"bar"}`, string(genericEvent.Payload()))
}

// TestGetRegisteredEventNames tests obtaining registered event names
func TestGetRegisteredEventNames(t *testing.T) {
	// Ensure there is at least one registered event
	RegisterEventType[*serializableEvent]()

	names := GetRegisteredEventNames()
	assert.Contains(t, names, "user.created")
}
