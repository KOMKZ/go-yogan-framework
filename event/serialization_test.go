package event

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// serializableEvent 测试用可序列化事件
// 使用公开字段，便于 JSON 序列化/反序列化
type serializableEvent struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
}

// Name 返回固定的事件名称
func (e *serializableEvent) Name() string {
	return "user.created"
}

func newSerializableEvent(userID int, username string) *serializableEvent {
	return &serializableEvent{
		UserID:   userID,
		Username: username,
	}
}

// TestSerializeEvent 测试事件序列化
func TestSerializeEvent(t *testing.T) {
	event := newSerializableEvent(123, "john")
	payload, err := SerializeEvent(event, "trace-123")

	require.NoError(t, err)
	assert.Equal(t, "user.created", payload.EventName)
	assert.Equal(t, "trace-123", payload.TraceID)
	assert.NotZero(t, payload.OccurredAt)

	// 验证 Payload 内容
	var data map[string]any
	err = json.Unmarshal(payload.Payload, &data)
	require.NoError(t, err)
	assert.Equal(t, float64(123), data["user_id"])
	assert.Equal(t, "john", data["username"])
}

// TestSerializeEvent_NoTraceID 测试无 TraceID
func TestSerializeEvent_NoTraceID(t *testing.T) {
	event := newSerializableEvent(456, "jane")
	payload, err := SerializeEvent(event, "")

	require.NoError(t, err)
	assert.Empty(t, payload.TraceID)
}

// TestDeserializeEvent_Registered 测试反序列化已注册事件
func TestDeserializeEvent_Registered(t *testing.T) {
	// 注册事件类型（使用指针类型）
	RegisterEventType[*serializableEvent]()

	// 创建 payload
	original := newSerializableEvent(789, "alice")
	payload, err := SerializeEvent(original, "trace-456")
	require.NoError(t, err)

	// 反序列化
	event, err := DeserializeEvent(payload)
	require.NoError(t, err)

	// 验证类型
	userEvent, ok := event.(*serializableEvent)
	require.True(t, ok)
	assert.Equal(t, 789, userEvent.UserID)
	assert.Equal(t, "alice", userEvent.Username)
}

// TestDeserializeEvent_Unregistered 测试反序列化未注册事件
func TestDeserializeEvent_Unregistered(t *testing.T) {
	payload := &KafkaEventPayload{
		EventName: "unknown.event",
		Payload:   json.RawMessage(`{"foo":"bar"}`),
	}

	event, err := DeserializeEvent(payload)
	require.NoError(t, err)

	// 应该返回 GenericEvent
	genericEvent, ok := event.(*GenericEvent)
	require.True(t, ok)
	assert.Equal(t, "unknown.event", genericEvent.Name())
	assert.JSONEq(t, `{"foo":"bar"}`, string(genericEvent.Payload()))
}

// TestGetRegisteredEventNames 测试获取已注册事件名称
func TestGetRegisteredEventNames(t *testing.T) {
	// 确保至少有一个注册的事件
	RegisterEventType[*serializableEvent]()

	names := GetRegisteredEventNames()
	assert.Contains(t, names, "user.created")
}
