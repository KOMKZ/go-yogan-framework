package event

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"time"
)

// KafkaEventPayload Kafka 事件消息格式
type KafkaEventPayload struct {
	EventName  string          `json:"event_name"`
	Payload    json.RawMessage `json:"payload"`
	OccurredAt time.Time       `json:"occurred_at"`
	TraceID    string          `json:"trace_id,omitempty"`
}

// SerializeEvent 序列化事件为 KafkaEventPayload
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

// eventRegistry 事件注册表（用于反序列化）
var (
	eventRegistry   = make(map[string]reflect.Type)
	eventRegistryMu sync.RWMutex
)

// RegisterEventType 注册事件类型（用于反序列化）
// 在应用启动时调用，注册所有需要从 Kafka 消费的事件类型
// T 应该是指针类型，如 RegisterEventType[*UserCreatedEvent]()
func RegisterEventType[T Event]() {
	typ := reflect.TypeOf((*T)(nil)).Elem()

	// 如果是指针类型，获取元素类型
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	// 创建一个零值实例来获取事件名称
	instance := reflect.New(typ).Interface().(Event)
	name := instance.Name()

	eventRegistryMu.Lock()
	defer eventRegistryMu.Unlock()
	eventRegistry[name] = typ
}

// DeserializeEvent 反序列化事件
func DeserializeEvent(payload *KafkaEventPayload) (Event, error) {
	eventRegistryMu.RLock()
	typ, ok := eventRegistry[payload.EventName]
	eventRegistryMu.RUnlock()

	if !ok {
		// 未注册的事件类型，返回 GenericEvent
		return &GenericEvent{
			name:    payload.EventName,
			payload: payload.Payload,
		}, nil
	}

	// 创建事件实例
	eventPtr := reflect.New(typ).Interface()
	if err := json.Unmarshal(payload.Payload, eventPtr); err != nil {
		return nil, fmt.Errorf("unmarshal event %s failed: %w", payload.EventName, err)
	}

	return eventPtr.(Event), nil
}

// GenericEvent 通用事件（用于未注册类型）
type GenericEvent struct {
	name    string
	payload json.RawMessage
}

// Name 返回事件名称
func (e *GenericEvent) Name() string {
	return e.name
}

// Payload 返回原始负载
func (e *GenericEvent) Payload() json.RawMessage {
	return e.payload
}

// GetRegisteredEventNames 获取已注册的事件名称列表
func GetRegisteredEventNames() []string {
	eventRegistryMu.RLock()
	defer eventRegistryMu.RUnlock()

	names := make([]string, 0, len(eventRegistry))
	for name := range eventRegistry {
		names = append(names, name)
	}
	return names
}
