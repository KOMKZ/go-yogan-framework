package event

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testEvent test event
type testEvent struct {
	name string
}

func (e *testEvent) Name() string {
	return e.name
}

// mockKafkaPublisher simulate Kafka publisher
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

// TestDispatchOption_WithKafka tests the WithKafka option
func TestDispatchOption_WithKafka(t *testing.T) {
	opts := &dispatchOptions{}
	WithKafka("test-topic")(opts)

	assert.Equal(t, DriverKafka, opts.driver)
	assert.Equal(t, "test-topic", opts.topic)
}

// TestDispatchOption_WithKafkaKey tests the WithKafkaKey option
func TestDispatchOption_WithKafkaKey(t *testing.T) {
	opts := &dispatchOptions{}
	WithKafkaKey("my-key")(opts)

	assert.Equal(t, "my-key", opts.key)
}

// TestDispatchOption_WithDispatchAsync tests the WithDispatchAsync option
func TestDispatchOption_WithDispatchAsync(t *testing.T) {
	opts := &dispatchOptions{}
	WithDispatchAsync()(opts)

	assert.True(t, opts.async)
}

// TestDispatchOptions_ApplyDefaults test default values
func TestDispatchOptions_ApplyDefaults(t *testing.T) {
	opts := &dispatchOptions{}
	opts.applyDefaults()

	assert.Equal(t, DriverMemory, opts.driver)
}

// TestDispatch_WithKafka test sending to Kafka
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
	assert.Equal(t, "user.created", messages[0].Key) // Use the event name as the key by default
}

// TestDispatch_WithKafka_CustomKey test custom key
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

// TestDispatch_WithKafka_NoPublisher tests scenario with no Kafka publisher configured
func TestDispatch_WithKafka_NoPublisher(t *testing.T) {
	d := NewDispatcher() // No KafkaPublisher
	defer d.Close()

	event := &testEvent{name: "test.event"}
	err := d.Dispatch(context.Background(), event, WithKafka("events.test"))

	assert.ErrorIs(t, err, ErrKafkaNotAvailable)
}

// TestDispatch_WithKafka_NoTopic tests no topic specified
func TestDispatch_WithKafka_NoTopic(t *testing.T) {
	publisher := &mockKafkaPublisher{}
	d := NewDispatcher(WithKafkaPublisher(publisher))
	defer d.Close()

	event := &testEvent{name: "test.event"}
	// Manually set driver to kafka but do not set topic
	err := d.Dispatch(context.Background(), event, func(o *dispatchOptions) {
		o.driver = DriverKafka
		// Do not set topic
	})

	assert.ErrorIs(t, err, ErrKafkaTopicRequired)
}

// TestDispatch_WithKafka_Async tests asynchronous sending to Kafka
func TestDispatch_WithKafka_Async(t *testing.T) {
	publisher := &mockKafkaPublisher{}
	d := NewDispatcher(WithKafkaPublisher(publisher))
	defer d.Close()

	event := &testEvent{name: "async.event"}
	err := d.Dispatch(context.Background(), event,
		WithKafka("events.async"),
		WithDispatchAsync())

	require.NoError(t, err)

	// wait for async completion
	time.Sleep(100 * time.Millisecond)

	messages := publisher.getMessages()
	require.Len(t, messages, 1)
	assert.Equal(t, "events.async", messages[0].Topic)
}

// TestDispatch_DefaultMemory test default memory route
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

// TestDispatch_MemoryAsync test memory async dispatch
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

	// await asynchronous completion
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	assert.True(t, called)
	mu.Unlock()
}

// TestDispatch_CombinedOptions test combined options
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

	// wait for asynchronous completion
	time.Sleep(100 * time.Millisecond)

	messages := publisher.getMessages()
	require.Len(t, messages, 1)
	assert.Equal(t, "events.combined", messages[0].Topic)
	assert.Equal(t, "key:456", messages[0].Key)
}
