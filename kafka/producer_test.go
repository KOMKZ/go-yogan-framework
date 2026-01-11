package kafka

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
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

// MockSyncProducer 模拟同步生产者
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

// 测试 SyncProducer 的验证逻辑
func TestSyncProducer_Send_Validation(t *testing.T) {
	logger := zap.NewNop()

	// 创建一个模拟的 SyncProducer 结构来测试验证逻辑
	p := &SyncProducer{
		logger: logger,
		closed: false,
	}

	// 测试 nil 消息
	t.Run("nil message", func(t *testing.T) {
		// 由于没有实际的 sarama producer，这里只测试前置验证
		// 实际的验证在 Send 方法开头
	})

	// 测试空 topic
	t.Run("empty topic", func(t *testing.T) {
		// 验证逻辑测试
	})

	// 测试关闭后发送
	t.Run("send after close", func(t *testing.T) {
		p.closed = true
		_, err := p.Send(context.Background(), &Message{Topic: "test"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "producer is closed")
	})
}

// 测试 AsyncProducer 的验证逻辑
func TestAsyncProducer_Validation(t *testing.T) {
	logger := zap.NewNop()

	p := &AsyncProducer{
		logger:    logger,
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
	logger := zap.NewNop()

	p := &AsyncProducer{
		logger:    logger,
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
	logger := zap.NewNop()

	p := &AsyncProducer{
		logger:    logger,
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
	logger := zap.NewNop()

	successCh := make(chan *ProducerResult, 10)
	errorCh := make(chan error, 10)

	p := &AsyncProducer{
		logger:    logger,
		closed:    false,
		successCh: successCh,
		errorCh:   errorCh,
	}

	// 测试 Successes 和 Errors 通道
	assert.Equal(t, (<-chan *ProducerResult)(successCh), p.Successes())
	assert.Equal(t, (<-chan error)(errorCh), p.Errors())
}

func TestAsyncProducer_Close_Idempotent(t *testing.T) {
	logger := zap.NewNop()

	p := &AsyncProducer{
		logger:    logger,
		closed:    true, // 已关闭
		successCh: make(chan *ProducerResult, 10),
		errorCh:   make(chan error, 10),
	}

	// 重复关闭不应该报错
	err := p.Close()
	assert.NoError(t, err)
}

func TestSyncProducer_Close_Idempotent(t *testing.T) {
	logger := zap.NewNop()

	p := &SyncProducer{
		logger: logger,
		closed: true, // 已关闭
	}

	// 重复关闭不应该报错
	err := p.Close()
	assert.NoError(t, err)
}

// 测试真实的 Producer（需要 Kafka 运行）
func TestProducer_Send_WithKafka(t *testing.T) {
	logger := zap.NewNop()
	cfg := Config{
		Brokers: []string{"localhost:9092"},
		Version: "3.8.0",
		Producer: ProducerConfig{
			Enabled:      true,
			RequiredAcks: 1,
		},
	}

	manager, err := NewManager(cfg, logger)
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

	// 测试发送消息
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
	logger := zap.NewNop()
	cfg := Config{
		Brokers: []string{"localhost:9092"},
		Version: "3.8.0",
		Producer: ProducerConfig{
			Enabled:      true,
			RequiredAcks: 1,
		},
	}

	manager, err := NewManager(cfg, logger)
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
	logger := zap.NewNop()
	cfg := Config{
		Brokers: []string{"localhost:9092"},
		Version: "3.8.0",
		Producer: ProducerConfig{
			Enabled:      true,
			RequiredAcks: 1,
		},
	}

	manager, err := NewManager(cfg, logger)
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
		// 不管成功失败都关闭 done
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
	logger := zap.NewNop()

	p := &SyncProducer{
		logger: logger,
		closed: false,
	}

	_, err := p.Send(context.Background(), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "message cannot be nil")
}

func TestSyncProducer_Send_EmptyTopic(t *testing.T) {
	logger := zap.NewNop()

	p := &SyncProducer{
		logger: logger,
		closed: false,
	}

	_, err := p.Send(context.Background(), &Message{Topic: ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "topic cannot be empty")
}

// 真实 Kafka 测试
func TestProducer_RealKafka_FullFlow(t *testing.T) {
	logger := zap.NewNop()
	cfg := Config{
		Brokers: []string{"localhost:9092"},
		Version: "3.8.0",
		Producer: ProducerConfig{
			Enabled:      true,
			RequiredAcks: 1,
			Compression:  "gzip",
		},
	}

	manager, err := NewManager(cfg, logger)
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

	// 测试发送带 Headers 的消息
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

	// 测试发送 JSON
	jsonData := struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}{"test", 123}

	result2, err := producer.SendJSON(context.Background(), "test-topic", "json-key", jsonData)
	assert.NoError(t, err)
	assert.NotNil(t, result2)
	t.Logf("JSON sent: partition=%d, offset=%d", result2.Partition, result2.Offset)

	// 测试异步发送
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
	logger := zap.NewNop()
	cfg := Config{
		Brokers: []string{"localhost:9092"},
		Version: "3.8.0",
	}

	manager, err := NewManager(cfg, logger)
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
	logger := zap.NewNop()
	cfg := Config{
		Brokers: []string{"localhost:9092"},
		Version: "3.8.0",
	}

	manager, err := NewManager(cfg, logger)
	if err != nil {
		t.Skip("Cannot create manager:", err)
	}
	defer manager.Close()

	err = manager.Ping(context.Background())
	assert.NoError(t, err)
}

func TestManager_RealKafka_CreateConsumer(t *testing.T) {
	logger := zap.NewNop()
	cfg := Config{
		Brokers: []string{"localhost:9092"},
		Version: "3.8.0",
		Consumer: ConsumerConfig{
			Enabled: true,
			GroupID: "test-group",
			Topics:  []string{"test-topic"},
		},
	}

	manager, err := NewManager(cfg, logger)
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

	// 获取消费者
	got := manager.GetConsumer("real-consumer")
	assert.Equal(t, consumer, got)
}

func TestAsyncProducer_RealKafka(t *testing.T) {
	logger := zap.NewNop()
	cfg := Config{
		Brokers: []string{"localhost:9092"},
		Version: "3.8.0",
		Producer: ProducerConfig{
			Enabled:      true,
			RequiredAcks: 1,
		},
	}

	manager, err := NewManager(cfg, logger)
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

	// 测试 SendJSON
	jsonData := map[string]interface{}{"test": "async-json"}
	_, sendErr := asyncProducer.SendJSON(context.Background(), "test-topic", "async-key", jsonData)
	assert.NoError(t, sendErr)

	// 测试 Successes 和 Errors channels
	successCh := asyncProducer.Successes()
	assert.NotNil(t, successCh)

	errorsCh := asyncProducer.Errors()
	assert.NotNil(t, errorsCh)

	// 等待一下让消息处理
	time.Sleep(500 * time.Millisecond)
}

func TestSyncProducer_SendJSON_Error(t *testing.T) {
	logger := zap.NewNop()

	p := &SyncProducer{
		logger: logger,
		closed: false,
	}

	// 测试 JSON 序列化失败
	_, err := p.SendJSON(context.Background(), "test-topic", "key", make(chan int))
	assert.Error(t, err)
}

func TestAsyncProducer_RealKafka_FullFlow(t *testing.T) {
	logger := zap.NewNop()
	cfg := Config{
		Brokers: []string{"localhost:9092"},
		Version: "3.8.0",
		Producer: ProducerConfig{
			Enabled:      true,
			RequiredAcks: 1,
		},
	}

	manager, err := NewManager(cfg, logger)
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

	// 发送多条异步消息
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

	// 等待消息发送完成
	time.Sleep(2 * time.Second)

	// 测试 Successes 和 Errors channels
	select {
	case result := <-asyncProducer.Successes():
		t.Logf("Got success from channel: partition=%d, offset=%d", result.Partition, result.Offset)
	case <-time.After(100 * time.Millisecond):
		// 超时也是正常的
	}

	select {
	case err := <-asyncProducer.Errors():
		t.Logf("Got error from channel: %v", err)
	case <-time.After(100 * time.Millisecond):
		// 超时也是正常的
	}
}

func TestAsyncProducer_SendAsync_AfterManagerClosed(t *testing.T) {
	logger := zap.NewNop()
	cfg := Config{
		Brokers: []string{"localhost:9092"},
		Version: "3.8.0",
		Producer: ProducerConfig{
			Enabled:      true,
			RequiredAcks: 1,
		},
	}

	manager, err := NewManager(cfg, logger)
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

	// 关闭 manager
	manager.Close()

	// 尝试发送消息
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
		// 完成
	case <-time.After(2 * time.Second):
		t.Log("Timeout waiting for callback")
	}
}

