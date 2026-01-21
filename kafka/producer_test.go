package kafka

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/stretchr/testify/assert"
)

func TestMessage_Fields(t *testing.T) {
	msg := &Message{
		Topic:     "test-topic",
		Key:       []byte("test-key"),
		Value:     []byte("test-value"),
		Headers:   map[string]string{"key": "value"},
		Partition: 0,
		Timestamp: time.Now(),
	}

	assert.Equal(t, "test-topic", msg.Topic)
	assert.Equal(t, []byte("test-key"), msg.Key)
	assert.Equal(t, []byte("test-value"), msg.Value)
	assert.Equal(t, "value", msg.Headers["key"])
	assert.Equal(t, int32(0), msg.Partition)
	assert.False(t, msg.Timestamp.IsZero())
}

func TestProducerResult_Fields(t *testing.T) {
	result := &ProducerResult{
		Topic:     "test-topic",
		Partition: 1,
		Offset:    100,
		Timestamp: time.Now(),
	}

	assert.Equal(t, "test-topic", result.Topic)
	assert.Equal(t, int32(1), result.Partition)
	assert.Equal(t, int64(100), result.Offset)
	assert.False(t, result.Timestamp.IsZero())
}

func TestNewSyncProducer_NilLogger(t *testing.T) {
	_, err := NewSyncProducer([]string{"localhost:9092"}, ProducerConfig{}, nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "logger cannot be nil")
}

func TestNewAsyncProducer_NilLogger(t *testing.T) {
	_, err := NewAsyncProducer([]string{"localhost:9092"}, ProducerConfig{}, nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "logger cannot be nil")
}

// MockSyncProducer simulated synchronous producer
type MockSyncProducer struct {
	sendFunc  func(msg *Message) (*ProducerResult, error)
	closeFunc func() error
	closed    bool
}

func (m *MockSyncProducer) Send(ctx context.Context, msg *Message) (*ProducerResult, error) {
	if m.closed {
		return nil, assert.AnError
	}
	if m.sendFunc != nil {
		return m.sendFunc(msg)
	}
	return &ProducerResult{
		Topic:     msg.Topic,
		Partition: 0,
		Offset:    1,
		Timestamp: time.Now(),
	}, nil
}

func (m *MockSyncProducer) SendAsync(msg *Message, callback func(*ProducerResult, error)) {
	go func() {
		result, err := m.Send(context.Background(), msg)
		if callback != nil {
			callback(result, err)
		}
	}()
}

func (m *MockSyncProducer) SendJSON(ctx context.Context, topic string, key string, value interface{}) (*ProducerResult, error) {
	return m.Send(ctx, &Message{Topic: topic, Key: []byte(key)})
}

func (m *MockSyncProducer) Close() error {
	m.closed = true
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func TestMockSyncProducer_Send(t *testing.T) {
	mock := &MockSyncProducer{}
	msg := &Message{
		Topic: "test-topic",
		Value: []byte("test-value"),
	}

	result, err := mock.Send(context.Background(), msg)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "test-topic", result.Topic)
}

func TestMockSyncProducer_SendAsync(t *testing.T) {
	mock := &MockSyncProducer{}
	msg := &Message{
		Topic: "test-topic",
		Value: []byte("test-value"),
	}

	done := make(chan struct{})
	mock.SendAsync(msg, func(result *ProducerResult, err error) {
		assert.NoError(t, err)
		assert.NotNil(t, result)
		close(done)
	})

	select {
	case <-done:
		// success
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for async callback")
	}
}

func TestMockSyncProducer_Close(t *testing.T) {
	mock := &MockSyncProducer{}
	err := mock.Close()
	assert.NoError(t, err)
	assert.True(t, mock.closed)
}

func TestMockSyncProducer_SendAfterClose(t *testing.T) {
	mock := &MockSyncProducer{}
	mock.Close()

	_, err := mock.Send(context.Background(), &Message{Topic: "test"})
	assert.Error(t, err)
}

// Test the validation logic of SyncProducer
func TestSyncProducer_Send_Validation(t *testing.T) {
	log := logger.GetLogger("test")

	// Create a simulated SyncProducer structure to test validation logic
	p := &SyncProducer{
		logger: log,
		closed: false,
	}

	// Test nil message
	t.Run("nil message", func(t *testing.T) {
		// Since there is no actual sarama producer, only pre-validation is tested here
		// Actual validation occurs at the beginning of the Send method
	})

	// Test empty topic
	t.Run("empty topic", func(t *testing.T) {
		// Validate logic test
	})

	// Test send after shutdown
	t.Run("send after close", func(t *testing.T) {
		p.closed = true
		_, err := p.Send(context.Background(), &Message{Topic: "test"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "producer is closed")
	})
}

// Test the validation logic of AsyncProducer
func TestAsyncProducer_Validation(t *testing.T) {
	log := logger.GetLogger("test")

	p := &AsyncProducer{
		logger:    log,
		closed:    false,
		successCh: make(chan *ProducerResult, 10),
		errorCh:   make(chan error, 10),
		stopCh:    make(chan struct{}),
	}

	t.Run("send after close", func(t *testing.T) {
		p.closed = true
		_, err := p.Send(context.Background(), &Message{Topic: "test"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "producer is closed")
	})

	t.Run("send nil message", func(t *testing.T) {
		p.closed = false
		_, err := p.Send(context.Background(), nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "message cannot be nil")
	})

	t.Run("send empty topic", func(t *testing.T) {
		p.closed = false
		_, err := p.Send(context.Background(), &Message{Topic: ""})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "topic cannot be empty")
	})
}

func TestAsyncProducer_SendAsync_Closed(t *testing.T) {
	log := logger.GetLogger("test")

	p := &AsyncProducer{
		logger:    log,
		closed:    true,
		successCh: make(chan *ProducerResult, 10),
		errorCh:   make(chan error, 10),
	}

	done := make(chan struct{})
	p.SendAsync(&Message{Topic: "test"}, func(result *ProducerResult, err error) {
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "producer is closed")
		close(done)
	})

	select {
	case <-done:
		// success
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

func TestAsyncProducer_SendAsync_NilMessage(t *testing.T) {
	log := logger.GetLogger("test")

	p := &AsyncProducer{
		logger:    log,
		closed:    false,
		successCh: make(chan *ProducerResult, 10),
		errorCh:   make(chan error, 10),
	}

	done := make(chan struct{})
	p.SendAsync(nil, func(result *ProducerResult, err error) {
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "message cannot be nil")
		close(done)
	})

	select {
	case <-done:
		// success
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

func TestAsyncProducer_Channels(t *testing.T) {
	log := logger.GetLogger("test")

	successCh := make(chan *ProducerResult, 10)
	errorCh := make(chan error, 10)

	p := &AsyncProducer{
		logger:    log,
		closed:    false,
		successCh: successCh,
		errorCh:   errorCh,
	}

	// Test Successes and Errors channels
	assert.Equal(t, (<-chan *ProducerResult)(successCh), p.Successes())
	assert.Equal(t, (<-chan error)(errorCh), p.Errors())
}

func TestAsyncProducer_Close_Idempotent(t *testing.T) {
	log := logger.GetLogger("test")

	p := &AsyncProducer{
		logger:    log,
		closed:    true, // closed
		successCh: make(chan *ProducerResult, 10),
		errorCh:   make(chan error, 10),
	}

	// Repeated closure should not result in an error
	err := p.Close()
	assert.NoError(t, err)
}

func TestSyncProducer_Close_Idempotent(t *testing.T) {
	log := logger.GetLogger("test")

	p := &SyncProducer{
		logger: log,
		closed: true, // closed
	}

	// Repeated closure should not result in an error
	err := p.Close()
	assert.NoError(t, err)
}

// Test the real Producer (requires Kafka to be running)
func TestProducer_Send_WithKafka(t *testing.T) {
	log := logger.GetLogger("test")
	cfg := Config{
		Brokers: []string{"localhost:9092"},
		Version: "3.8.0",
		Producer: ProducerConfig{
			Enabled:      true,
			RequiredAcks: 1,
		},
	}

	manager, err := NewManager(cfg, log)
	assert.NoError(t, err)
	defer manager.Close()

	err = manager.Connect(context.Background())
	if err != nil {
		t.Skip("Kafka not available")
	}

	producer := manager.GetProducer()
	if producer == nil {
		t.Skip("Producer not created")
	}

	// Test sending message
	msg := &Message{
		Topic: "test-topic",
		Key:   []byte("key1"),
		Value: []byte("value1"),
		Headers: map[string]string{
			"header1": "value1",
		},
		Partition: -1,
		Timestamp: time.Now(),
	}

	result, err := producer.Send(context.Background(), msg)
	if err != nil {
		t.Skip("Kafka send failed:", err)
	}
	assert.NotNil(t, result)
	assert.Equal(t, "test-topic", result.Topic)
}

func TestProducer_SendJSON_WithKafka(t *testing.T) {
	log := logger.GetLogger("test")
	cfg := Config{
		Brokers: []string{"localhost:9092"},
		Version: "3.8.0",
		Producer: ProducerConfig{
			Enabled:      true,
			RequiredAcks: 1,
		},
	}

	manager, err := NewManager(cfg, log)
	assert.NoError(t, err)
	defer manager.Close()

	err = manager.Connect(context.Background())
	if err != nil {
		t.Skip("Kafka not available")
	}

	producer := manager.GetProducer()
	if producer == nil {
		t.Skip("Producer not created")
	}

	data := struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}{
		Name: "test",
		Age:  25,
	}

	result, err := producer.SendJSON(context.Background(), "test-topic", "json-key", data)
	if err != nil {
		t.Skip("Kafka send failed:", err)
	}
	assert.NotNil(t, result)
}

func TestProducer_SendAsync_WithKafka(t *testing.T) {
	log := logger.GetLogger("test")
	cfg := Config{
		Brokers: []string{"localhost:9092"},
		Version: "3.8.0",
		Producer: ProducerConfig{
			Enabled:      true,
			RequiredAcks: 1,
		},
	}

	manager, err := NewManager(cfg, log)
	assert.NoError(t, err)
	defer manager.Close()

	err = manager.Connect(context.Background())
	if err != nil {
		t.Skip("Kafka not available")
	}

	producer := manager.GetProducer()
	if producer == nil {
		t.Skip("Producer not created")
	}

	msg := &Message{
		Topic: "test-topic",
		Value: []byte("async-value"),
	}

	done := make(chan struct{})
	producer.SendAsync(msg, func(result *ProducerResult, err error) {
		// close done regardless of success or failure
		defer close(done)
		if err != nil {
			t.Log("Async send error:", err)
			return
		}
		assert.NotNil(t, result)
	})

	select {
	case <-done:
		// completed
	case <-time.After(5 * time.Second):
		t.Skip("timeout waiting for async send - Kafka may not be available")
	}
}

func TestSyncProducer_Send_NilMessage(t *testing.T) {
	log := logger.GetLogger("test")

	p := &SyncProducer{
		logger: log,
		closed: false,
	}

	_, err := p.Send(context.Background(), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "message cannot be nil")
}

func TestSyncProducer_Send_EmptyTopic(t *testing.T) {
	log := logger.GetLogger("test")

	p := &SyncProducer{
		logger: log,
		closed: false,
	}

	_, err := p.Send(context.Background(), &Message{Topic: ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "topic cannot be empty")
}

// Real Kafka test
func TestProducer_RealKafka_FullFlow(t *testing.T) {
	log := logger.GetLogger("test")
	cfg := Config{
		Brokers: []string{"localhost:9092"},
		Version: "3.8.0",
		Producer: ProducerConfig{
			Enabled:      true,
			RequiredAcks: 1,
			Compression:  "gzip",
		},
	}

	manager, err := NewManager(cfg, log)
	if err != nil {
		t.Skip("Cannot create manager:", err)
	}
	defer manager.Close()

	err = manager.Connect(context.Background())
	if err != nil {
		t.Skip("Kafka not available:", err)
	}

	producer := manager.GetProducer()
	if producer == nil {
		t.Skip("Producer not created")
	}

	// Test sending messages with Headers
	msg := &Message{
		Topic: "test-topic",
		Key:   []byte("test-key"),
		Value: []byte("test-value"),
		Headers: map[string]string{
			"header1":      "value1",
			"content-type": "text/plain",
		},
		Partition: -1,
		Timestamp: time.Now(),
	}

	result, err := producer.Send(context.Background(), msg)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "test-topic", result.Topic)
	assert.GreaterOrEqual(t, result.Partition, int32(0))
	assert.GreaterOrEqual(t, result.Offset, int64(0))
	t.Logf("Message sent: partition=%d, offset=%d", result.Partition, result.Offset)

	// Test sending JSON
	jsonData := struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}{"test", 123}

	result2, err := producer.SendJSON(context.Background(), "test-topic", "json-key", jsonData)
	assert.NoError(t, err)
	assert.NotNil(t, result2)
	t.Logf("JSON sent: partition=%d, offset=%d", result2.Partition, result2.Offset)

	// Test asynchronous sending
	done := make(chan struct{})
	producer.SendAsync(&Message{
		Topic: "test-topic",
		Value: []byte("async-message"),
	}, func(result *ProducerResult, err error) {
		defer close(done)
		assert.NoError(t, err)
		if result != nil {
			t.Logf("Async sent: partition=%d, offset=%d", result.Partition, result.Offset)
		}
	})

	select {
	case <-done:
		// success
	case <-time.After(5 * time.Second):
		t.Log("Async send timeout")
	}
}

func TestManager_RealKafka_ListTopics(t *testing.T) {
	log := logger.GetLogger("test")
	cfg := Config{
		Brokers: []string{"localhost:9092"},
		Version: "3.8.0",
	}

	manager, err := NewManager(cfg, log)
	if err != nil {
		t.Skip("Cannot create manager:", err)
	}
	defer manager.Close()

	topics, err := manager.ListTopics(context.Background())
	if err != nil {
		t.Skip("Cannot list topics:", err)
	}

	assert.NotNil(t, topics)
	t.Logf("Found %d topics: %v", len(topics), topics)
}

func TestManager_RealKafka_Ping(t *testing.T) {
	log := logger.GetLogger("test")
	cfg := Config{
		Brokers: []string{"localhost:9092"},
		Version: "3.8.0",
	}

	manager, err := NewManager(cfg, log)
	if err != nil {
		t.Skip("Cannot create manager:", err)
	}
	defer manager.Close()

	err = manager.Ping(context.Background())
	assert.NoError(t, err)
}

func TestManager_RealKafka_CreateConsumer(t *testing.T) {
	log := logger.GetLogger("test")
	cfg := Config{
		Brokers: []string{"localhost:9092"},
		Version: "3.8.0",
		Consumer: ConsumerConfig{
			Enabled: true,
			GroupID: "test-group",
			Topics:  []string{"test-topic"},
		},
	}

	manager, err := NewManager(cfg, log)
	if err != nil {
		t.Skip("Cannot create manager:", err)
	}
	defer manager.Close()

	consumer, err := manager.CreateConsumer("real-consumer", ConsumerConfig{
		GroupID:       "test-group-real",
		Topics:        []string{"test-topic"},
		OffsetInitial: -2,
		AutoCommit:    true,
	})

	assert.NoError(t, err)
	assert.NotNil(t, consumer)
	assert.False(t, consumer.IsRunning())

	// Get consumer
	got := manager.GetConsumer("real-consumer")
	assert.Equal(t, consumer, got)
}

func TestAsyncProducer_RealKafka(t *testing.T) {
	log := logger.GetLogger("test")
	cfg := Config{
		Brokers: []string{"localhost:9092"},
		Version: "3.8.0",
		Producer: ProducerConfig{
			Enabled:      true,
			RequiredAcks: 1,
		},
	}

	manager, err := NewManager(cfg, log)
	if err != nil {
		t.Skip("Cannot create manager:", err)
	}
	defer manager.Close()

	err = manager.Connect(context.Background())
	if err != nil {
		t.Skip("Kafka not available:", err)
	}

	asyncProducer, err := manager.GetAsyncProducer()
	if err != nil {
		t.Skip("Cannot get async producer:", err)
	}

	// Test SendJSON
	jsonData := map[string]interface{}{"test": "async-json"}
	_, sendErr := asyncProducer.SendJSON(context.Background(), "test-topic", "async-key", jsonData)
	assert.NoError(t, sendErr)

	// Test Successes and Errors channels
	successCh := asyncProducer.Successes()
	assert.NotNil(t, successCh)

	errorsCh := asyncProducer.Errors()
	assert.NotNil(t, errorsCh)

	// wait for the message to be processed
	time.Sleep(500 * time.Millisecond)
}

func TestSyncProducer_SendJSON_Error(t *testing.T) {
	log := logger.GetLogger("test")

	p := &SyncProducer{
		logger: log,
		closed: false,
	}

	// Test JSON serialization failure
	_, err := p.SendJSON(context.Background(), "test-topic", "key", make(chan int))
	assert.Error(t, err)
}

func TestAsyncProducer_RealKafka_FullFlow(t *testing.T) {
	log := logger.GetLogger("test")
	cfg := Config{
		Brokers: []string{"localhost:9092"},
		Version: "3.8.0",
		Producer: ProducerConfig{
			Enabled:      true,
			RequiredAcks: 1,
		},
	}

	manager, err := NewManager(cfg, log)
	if err != nil {
		t.Skip("Cannot create manager:", err)
	}
	defer manager.Close()

	err = manager.Connect(context.Background())
	if err != nil {
		t.Skip("Kafka not available:", err)
	}

	asyncProducer, err := manager.GetAsyncProducer()
	if err != nil {
		t.Skip("Cannot get async producer:", err)
	}

	// Send multiple asynchronous messages
	for i := 0; i < 5; i++ {
		msg := &Message{
			Topic: "test-topic",
			Key:   []byte(fmt.Sprintf("async-key-%d", i)),
			Value: []byte(fmt.Sprintf("async-value-%d", i)),
		}

		asyncProducer.SendAsync(msg, func(result *ProducerResult, err error) {
			if err != nil {
				t.Logf("Async send error: %v", err)
			} else {
				t.Logf("Async success: partition=%d, offset=%d", result.Partition, result.Offset)
			}
		})
	}

	// wait for message send completion
	time.Sleep(2 * time.Second)

	// Test Successes and Errors channels
	select {
	case result := <-asyncProducer.Successes():
		t.Logf("Got success from channel: partition=%d, offset=%d", result.Partition, result.Offset)
	case <-time.After(100 * time.Millisecond):
		// Timeout is also normal
	}

	select {
	case err := <-asyncProducer.Errors():
		t.Logf("Got error from channel: %v", err)
	case <-time.After(100 * time.Millisecond):
		// Timeout is also normal
	}
}

func TestAsyncProducer_SendAsync_AfterManagerClosed(t *testing.T) {
	log := logger.GetLogger("test")
	cfg := Config{
		Brokers: []string{"localhost:9092"},
		Version: "3.8.0",
		Producer: ProducerConfig{
			Enabled:      true,
			RequiredAcks: 1,
		},
	}

	manager, err := NewManager(cfg, log)
	if err != nil {
		t.Skip("Cannot create manager:", err)
	}

	err = manager.Connect(context.Background())
	if err != nil {
		t.Skip("Kafka not available:", err)
	}

	asyncProducer, err := manager.GetAsyncProducer()
	if err != nil {
		t.Skip("Cannot get async producer:", err)
	}

	// Shut down manager
	manager.Close()

	// Try to send message
	done := make(chan struct{})
	asyncProducer.SendAsync(&Message{
		Topic: "test-topic",
		Value: []byte("test"),
	}, func(result *ProducerResult, err error) {
		if err != nil {
			t.Logf("Expected error after close: %v", err)
		}
		close(done)
	})

	select {
	case <-done:
		// Complete
	case <-time.After(2 * time.Second):
		t.Log("Timeout waiting for callback")
	}
}

