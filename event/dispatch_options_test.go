package event

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testEvent 测试用事件
type testEvent struct {
	name string
}

func (e *testEvent) Name() string {
	return e.name
}

// mockKafkaPublisher 模拟 Kafka 发布者
type mockKafkaPublisher struct {
	mu       sync.Mutex
	messages []mockKafkaMessage
	err      error
}

type mockKafkaMessage struct {
	Topic   string
	Key     string
	Payload any
}

func (m *mockKafkaPublisher) PublishJSON(ctx context.Context, topic string, key string, payload any) error {
	if m.err != nil {
		return m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, mockKafkaMessage{
		Topic:   topic,
		Key:     key,
		Payload: payload,
	})
	return nil
}

func (m *mockKafkaPublisher) getMessages() []mockKafkaMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]mockKafkaMessage{}, m.messages...)
}

// TestDispatchOption_WithKafka 测试 WithKafka 选项
func TestDispatchOption_WithKafka(t *testing.T) {
	opts := &dispatchOptions{}
	WithKafka("test-topic")(opts)

	assert.Equal(t, DriverKafka, opts.driver)
	assert.Equal(t, "test-topic", opts.topic)
}

// TestDispatchOption_WithKafkaKey 测试 WithKafkaKey 选项
func TestDispatchOption_WithKafkaKey(t *testing.T) {
	opts := &dispatchOptions{}
	WithKafkaKey("my-key")(opts)

	assert.Equal(t, "my-key", opts.key)
}

// TestDispatchOption_WithDispatchAsync 测试 WithDispatchAsync 选项
func TestDispatchOption_WithDispatchAsync(t *testing.T) {
	opts := &dispatchOptions{}
	WithDispatchAsync()(opts)

	assert.True(t, opts.async)
}

// TestDispatchOptions_ApplyDefaults 测试默认值
func TestDispatchOptions_ApplyDefaults(t *testing.T) {
	opts := &dispatchOptions{}
	opts.applyDefaults()

	assert.Equal(t, DriverMemory, opts.driver)
}

// TestDispatch_WithKafka 测试发送到 Kafka
func TestDispatch_WithKafka(t *testing.T) {
	publisher := &mockKafkaPublisher{}
	d := NewDispatcher(WithKafkaPublisher(publisher))
	defer d.Close()

	event := &testEvent{name: "user.created"}
	err := d.Dispatch(context.Background(), event, WithKafka("events.user"))

	require.NoError(t, err)
	messages := publisher.getMessages()
	require.Len(t, messages, 1)
	assert.Equal(t, "events.user", messages[0].Topic)
	assert.Equal(t, "user.created", messages[0].Key) // 默认使用事件名作为 key
}

// TestDispatch_WithKafka_CustomKey 测试自定义 Key
func TestDispatch_WithKafka_CustomKey(t *testing.T) {
	publisher := &mockKafkaPublisher{}
	d := NewDispatcher(WithKafkaPublisher(publisher))
	defer d.Close()

	event := &testEvent{name: "order.created"}
	err := d.Dispatch(context.Background(), event,
		WithKafka("events.order"),
		WithKafkaKey("order:123"))

	require.NoError(t, err)
	messages := publisher.getMessages()
	require.Len(t, messages, 1)
	assert.Equal(t, "order:123", messages[0].Key)
}

// TestDispatch_WithKafka_NoPublisher 测试未配置 Kafka 发布者
func TestDispatch_WithKafka_NoPublisher(t *testing.T) {
	d := NewDispatcher() // 无 KafkaPublisher
	defer d.Close()

	event := &testEvent{name: "test.event"}
	err := d.Dispatch(context.Background(), event, WithKafka("events.test"))

	assert.ErrorIs(t, err, ErrKafkaNotAvailable)
}

// TestDispatch_WithKafka_NoTopic 测试未指定 Topic
func TestDispatch_WithKafka_NoTopic(t *testing.T) {
	publisher := &mockKafkaPublisher{}
	d := NewDispatcher(WithKafkaPublisher(publisher))
	defer d.Close()

	event := &testEvent{name: "test.event"}
	// 手动设置 driver 为 kafka 但不设置 topic
	err := d.Dispatch(context.Background(), event, func(o *dispatchOptions) {
		o.driver = DriverKafka
		// 不设置 topic
	})

	assert.ErrorIs(t, err, ErrKafkaTopicRequired)
}

// TestDispatch_WithKafka_Async 测试异步发送到 Kafka
func TestDispatch_WithKafka_Async(t *testing.T) {
	publisher := &mockKafkaPublisher{}
	d := NewDispatcher(WithKafkaPublisher(publisher))
	defer d.Close()

	event := &testEvent{name: "async.event"}
	err := d.Dispatch(context.Background(), event,
		WithKafka("events.async"),
		WithDispatchAsync())

	require.NoError(t, err)

	// 等待异步完成
	time.Sleep(100 * time.Millisecond)

	messages := publisher.getMessages()
	require.Len(t, messages, 1)
	assert.Equal(t, "events.async", messages[0].Topic)
}

// TestDispatch_DefaultMemory 测试默认走内存
func TestDispatch_DefaultMemory(t *testing.T) {
	d := NewDispatcher()
	defer d.Close()

	var called bool
	d.Subscribe("memory.event", ListenerFunc(func(ctx context.Context, e Event) error {
		called = true
		return nil
	}))

	event := &testEvent{name: "memory.event"}
	err := d.Dispatch(context.Background(), event)

	require.NoError(t, err)
	assert.True(t, called)
}

// TestDispatch_MemoryAsync 测试内存异步分发
func TestDispatch_MemoryAsync(t *testing.T) {
	d := NewDispatcher()
	defer d.Close()

	var called bool
	var mu sync.Mutex
	d.Subscribe("async.memory", ListenerFunc(func(ctx context.Context, e Event) error {
		mu.Lock()
		called = true
		mu.Unlock()
		return nil
	}))

	event := &testEvent{name: "async.memory"}
	err := d.Dispatch(context.Background(), event, WithDispatchAsync())

	require.NoError(t, err)

	// 等待异步完成
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	assert.True(t, called)
	mu.Unlock()
}

// TestDispatch_CombinedOptions 测试组合选项
func TestDispatch_CombinedOptions(t *testing.T) {
	publisher := &mockKafkaPublisher{}
	d := NewDispatcher(WithKafkaPublisher(publisher))
	defer d.Close()

	event := &testEvent{name: "combined.event"}
	err := d.Dispatch(context.Background(), event,
		WithKafka("events.combined"),
		WithKafkaKey("key:456"),
		WithDispatchAsync())

	require.NoError(t, err)

	// 等待异步完成
	time.Sleep(100 * time.Millisecond)

	messages := publisher.getMessages()
	require.Len(t, messages, 1)
	assert.Equal(t, "events.combined", messages[0].Topic)
	assert.Equal(t, "key:456", messages[0].Key)
}
