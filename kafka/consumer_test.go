package kafka

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestConsumedMessage_Fields(t *testing.T) {
	msg := &ConsumedMessage{
		Topic:     "test-topic",
		Partition: 1,
		Offset:    100,
		Key:       []byte("key"),
		Value:     []byte("value"),
		Headers:   map[string]string{"h1": "v1"},
		Timestamp: 1234567890,
	}

	assert.Equal(t, "test-topic", msg.Topic)
	assert.Equal(t, int32(1), msg.Partition)
	assert.Equal(t, int64(100), msg.Offset)
	assert.Equal(t, []byte("key"), msg.Key)
	assert.Equal(t, []byte("value"), msg.Value)
	assert.Equal(t, "v1", msg.Headers["h1"])
	assert.Equal(t, int64(1234567890), msg.Timestamp)
}

func TestNewConsumerGroup_NilLogger(t *testing.T) {
	cfg := ConsumerConfig{
		GroupID: "test-group",
		Topics:  []string{"test-topic"},
	}

	_, err := NewConsumerGroup([]string{"localhost:9092"}, cfg, nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "logger cannot be nil")
}

func TestNewConsumerGroup_EmptyGroupID(t *testing.T) {
	logger := zap.NewNop()
	cfg := ConsumerConfig{
		GroupID: "",
		Topics:  []string{"test-topic"},
	}

	_, err := NewConsumerGroup([]string{"localhost:9092"}, cfg, nil, logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "group_id cannot be empty")
}

func TestNewSimpleConsumer_NilLogger(t *testing.T) {
	_, err := NewSimpleConsumer([]string{"localhost:9092"}, nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "logger cannot be nil")
}

func TestConsumerGroup_IsRunning(t *testing.T) {
	logger := zap.NewNop()

	cg := &ConsumerGroup{
		logger:  logger,
		running: false,
		stopCh:  make(chan struct{}),
		doneCh:  make(chan struct{}),
	}

	assert.False(t, cg.IsRunning())

	cg.running = true
	assert.True(t, cg.IsRunning())
}

func TestConsumerGroup_Stop_NotRunning(t *testing.T) {
	logger := zap.NewNop()

	cg := &ConsumerGroup{
		logger:  logger,
		running: false,
		stopCh:  make(chan struct{}),
		doneCh:  make(chan struct{}),
	}

	err := cg.Stop()
	assert.NoError(t, err)
}

func TestConsumerGroup_Start_AlreadyRunning(t *testing.T) {
	logger := zap.NewNop()

	cg := &ConsumerGroup{
		logger:  logger,
		running: true,
		stopCh:  make(chan struct{}),
		doneCh:  make(chan struct{}),
	}

	err := cg.Start(context.Background(), func(ctx context.Context, msg *ConsumedMessage) error {
		return nil
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "consumer is already running")
}

func TestSimpleConsumer_IsRunning(t *testing.T) {
	logger := zap.NewNop()

	sc := &SimpleConsumer{
		logger:  logger,
		running: false,
		stopCh:  make(chan struct{}),
	}

	assert.False(t, sc.IsRunning())

	sc.running = true
	assert.True(t, sc.IsRunning())
}

func TestSimpleConsumer_ConsumePartition_AlreadyRunning(t *testing.T) {
	logger := zap.NewNop()

	sc := &SimpleConsumer{
		logger:  logger,
		running: true,
		stopCh:  make(chan struct{}),
	}

	err := sc.ConsumePartition(context.Background(), "topic", 0, 0, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "consumer is already running")
}

func TestSimpleConsumer_Stop_NotRunning(t *testing.T) {
	logger := zap.NewNop()

	sc := &SimpleConsumer{
		logger:  logger,
		running: false,
		stopCh:  make(chan struct{}),
	}

	err := sc.Stop()
	assert.NoError(t, err)
}

// 注意：consumerGroupHandler 的 Setup/Cleanup/ConsumeClaim 需要真实的 session
// 这些方法的测试依赖集成测试环境

// MockMessageHandler 用于测试
type MockMessageHandler struct {
	messages []*ConsumedMessage
	err      error
}

func (m *MockMessageHandler) Handle(ctx context.Context, msg *ConsumedMessage) error {
	m.messages = append(m.messages, msg)
	return m.err
}

func TestMockMessageHandler(t *testing.T) {
	handler := &MockMessageHandler{}

	msg := &ConsumedMessage{
		Topic: "test",
		Value: []byte("test-value"),
	}

	err := handler.Handle(context.Background(), msg)
	assert.NoError(t, err)
	assert.Len(t, handler.messages, 1)
	assert.Equal(t, msg, handler.messages[0])
}

func TestMockMessageHandler_WithError(t *testing.T) {
	handler := &MockMessageHandler{
		err: assert.AnError,
	}

	msg := &ConsumedMessage{
		Topic: "test",
		Value: []byte("test-value"),
	}

	err := handler.Handle(context.Background(), msg)
	assert.Error(t, err)
}

// 真实 Kafka 消费者测试
func TestConsumerGroup_RealKafka(t *testing.T) {
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

	consumer, err := manager.CreateConsumer("test-consumer", ConsumerConfig{
		GroupID:            "test-group-" + time.Now().Format("150405"),
		Topics:             []string{"test-topic"},
		OffsetInitial:      -1, // Newest
		AutoCommit:         true,
		AutoCommitInterval: 1 * time.Second,
	})
	if err != nil {
		t.Skip("Cannot create consumer:", err)
	}

	// 测试启动消费者
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	messageReceived := make(chan struct{})
	err = consumer.Start(ctx, func(ctx context.Context, msg *ConsumedMessage) error {
		t.Logf("Received message: topic=%s, partition=%d, offset=%d, value=%s",
			msg.Topic, msg.Partition, msg.Offset, string(msg.Value))
		select {
		case messageReceived <- struct{}{}:
		default:
		}
		return nil
	})
	assert.NoError(t, err)
	assert.True(t, consumer.IsRunning())

	// 等待一段时间
	select {
	case <-messageReceived:
		t.Log("Message received!")
	case <-time.After(2 * time.Second):
		t.Log("No message received (normal if topic is empty)")
	}

	// 停止消费者
	err = consumer.Stop()
	assert.NoError(t, err)
	assert.False(t, consumer.IsRunning())
}

func TestSimpleConsumer_RealKafka(t *testing.T) {
	logger := zap.NewNop()

	// 创建 sarama 配置
	saramaCfg := sarama.NewConfig()
	saramaCfg.Version = sarama.V3_0_0_0
	saramaCfg.Consumer.Return.Errors = true

	simpleConsumer, err := NewSimpleConsumer([]string{"localhost:9092"}, saramaCfg, logger)
	if err != nil {
		t.Skip("Cannot create simple consumer:", err)
	}

	assert.NotNil(t, simpleConsumer)
	assert.False(t, simpleConsumer.IsRunning())

	// 消费分区
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		err := simpleConsumer.ConsumePartition(ctx, "test-topic", 0, -2, func(ctx context.Context, msg *ConsumedMessage) error {
			t.Logf("Simple consumer received: %s", string(msg.Value))
			return nil
		})
		if err != nil && err != context.DeadlineExceeded {
			t.Log("ConsumePartition error:", err)
		}
	}()

	// 等待一段时间
	<-ctx.Done()

	// 停止
	err = simpleConsumer.Stop()
	assert.NoError(t, err)
}

func TestConsumerGroup_Stop_NotRunning_RealKafka(t *testing.T) {
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

	consumer, err := manager.CreateConsumer("stop-test", ConsumerConfig{
		GroupID: "stop-test-group",
		Topics:  []string{"test-topic"},
	})
	if err != nil {
		t.Skip("Cannot create consumer:", err)
	}

	// 停止未运行的消费者
	err = consumer.Stop()
	assert.NoError(t, err)
}

// 端到端测试：发送消息并消费
func TestConsumerGroup_ConsumeWithHandlerError(t *testing.T) {
	logger := zap.NewNop()
	cfg := Config{
		Brokers: []string{"localhost:9092"},
		Version: "3.8.0",
		Producer: ProducerConfig{
			Enabled:      true,
			RequiredAcks: 1,
		},
		Consumer: ConsumerConfig{
			Enabled: true,
			GroupID: "default-group",
			Topics:  []string{"test-topic"},
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

	groupID := "error-handler-group-" + time.Now().Format("150405999")
	consumer, err := manager.CreateConsumer("error-handler-consumer", ConsumerConfig{
		GroupID:       groupID,
		Topics:        []string{"test-topic"},
		OffsetInitial: -1, // Newest
		AutoCommit:    true,
	})
	assert.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 启动消费者，handler 返回错误
	err = consumer.Start(ctx, func(ctx context.Context, msg *ConsumedMessage) error {
		t.Logf("Received message, returning error")
		return fmt.Errorf("handler error")
	})
	assert.NoError(t, err)

	// 发送消息触发 handler
	producer := manager.GetProducer()
	_, err = producer.Send(context.Background(), &Message{
		Topic: "test-topic",
		Value: []byte("trigger-error"),
	})
	assert.NoError(t, err)

	// 等待一段时间
	time.Sleep(3 * time.Second)

	cancel()
	consumer.Stop()
}

func TestProducerConsumer_EndToEnd(t *testing.T) {
	logger := zap.NewNop()
	testTopic := "test-topic" // 使用已存在的 topic

	cfg := Config{
		Brokers: []string{"localhost:9092"},
		Version: "3.8.0",
		Producer: ProducerConfig{
			Enabled:      true,
			RequiredAcks: 1,
		},
		Consumer: ConsumerConfig{
			Enabled: true,
			GroupID: "default-group",
			Topics:  []string{testTopic},
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

	// 创建唯一的消费者组
	groupID := "e2e-group-" + time.Now().Format("150405999")

	// 1. 先启动消费者
	consumer, err := manager.CreateConsumer("e2e-consumer", ConsumerConfig{
		GroupID:            groupID,
		Topics:             []string{testTopic},
		OffsetInitial:      -1, // Newest - 只消费新消息
		AutoCommit:         true,
		AutoCommitInterval: 100 * time.Millisecond,
	})
	assert.NoError(t, err)

	received := make([]string, 0)
	var mu sync.Mutex
	testMessages := []string{"e2e-msg1", "e2e-msg2", "e2e-msg3"}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// 启动消费者
	err = consumer.Start(ctx, func(ctx context.Context, msg *ConsumedMessage) error {
		mu.Lock()
		received = append(received, string(msg.Value))
		t.Logf("Received: topic=%s, partition=%d, offset=%d, value=%s, headers=%v",
			msg.Topic, msg.Partition, msg.Offset, string(msg.Value), msg.Headers)
		cnt := len(received)
		mu.Unlock()

		// 收到足够消息后停止
		if cnt >= len(testMessages) {
			cancel()
		}
		return nil
	})
	assert.NoError(t, err)

	// 等待消费者准备好（分配分区）
	time.Sleep(3 * time.Second)

	// 2. 发送消息
	producer := manager.GetProducer()
	for i, msg := range testMessages {
		result, err := producer.Send(context.Background(), &Message{
			Topic: testTopic,
			Key:   []byte("key-" + string(rune('a'+i))),
			Value: []byte(msg),
			Headers: map[string]string{
				"test-header": "test-value",
			},
		})
		assert.NoError(t, err)
		t.Logf("Sent message %d: partition=%d, offset=%d", i, result.Partition, result.Offset)
	}

	// 等待消费完成或超时
	<-ctx.Done()

	// 停止消费者
	err = consumer.Stop()
	assert.NoError(t, err)

	// 验证
	mu.Lock()
	t.Logf("Received %d messages: %v", len(received), received)
	mu.Unlock()
}

