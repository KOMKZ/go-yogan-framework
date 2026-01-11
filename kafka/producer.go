package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"go.uber.org/zap"
)

// Message 消息结构
type Message struct {
	// Topic 目标 Topic
	Topic string

	// Key 消息键（用于分区）
	Key []byte

	// Value 消息值
	Value []byte

	// Headers 消息头
	Headers map[string]string

	// Partition 指定分区（-1 表示自动分配）
	Partition int32

	// Timestamp 消息时间戳
	Timestamp time.Time
}

// ProducerResult 发送结果
type ProducerResult struct {
	// Topic 发送的 Topic
	Topic string

	// Partition 发送的分区
	Partition int32

	// Offset 消息 Offset
	Offset int64

	// Timestamp 服务端时间戳
	Timestamp time.Time
}

// Producer Kafka 生产者接口
type Producer interface {
	// Send 同步发送消息
	Send(ctx context.Context, msg *Message) (*ProducerResult, error)

	// SendAsync 异步发送消息
	SendAsync(msg *Message, callback func(*ProducerResult, error))

	// SendJSON 发送 JSON 消息
	SendJSON(ctx context.Context, topic string, key string, value interface{}) (*ProducerResult, error)

	// Close 关闭生产者
	Close() error
}

// SyncProducer 同步生产者实现
type SyncProducer struct {
	producer sarama.SyncProducer
	config   ProducerConfig
	logger   *zap.Logger
	mu       sync.RWMutex
	closed   bool
}

// NewSyncProducer 创建同步生产者
func NewSyncProducer(brokers []string, cfg ProducerConfig, saramaCfg *sarama.Config, logger *zap.Logger) (*SyncProducer, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	producer, err := sarama.NewSyncProducer(brokers, saramaCfg)
	if err != nil {
		return nil, fmt.Errorf("create sync producer failed: %w", err)
	}

	return &SyncProducer{
		producer: producer,
		config:   cfg,
		logger:   logger,
	}, nil
}

// Send 同步发送消息
func (p *SyncProducer) Send(ctx context.Context, msg *Message) (*ProducerResult, error) {
	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		return nil, fmt.Errorf("producer is closed")
	}
	p.mu.RUnlock()

	if msg == nil {
		return nil, fmt.Errorf("message cannot be nil")
	}

	if msg.Topic == "" {
		return nil, fmt.Errorf("topic cannot be empty")
	}

	// 构建 Sarama 消息
	saramaMsg := &sarama.ProducerMessage{
		Topic: msg.Topic,
		Value: sarama.ByteEncoder(msg.Value),
	}

	if len(msg.Key) > 0 {
		saramaMsg.Key = sarama.ByteEncoder(msg.Key)
	}

	if msg.Partition >= 0 {
		saramaMsg.Partition = msg.Partition
	}

	if !msg.Timestamp.IsZero() {
		saramaMsg.Timestamp = msg.Timestamp
	}

	// 添加 Headers
	if len(msg.Headers) > 0 {
		headers := make([]sarama.RecordHeader, 0, len(msg.Headers))
		for k, v := range msg.Headers {
			headers = append(headers, sarama.RecordHeader{
				Key:   []byte(k),
				Value: []byte(v),
			})
		}
		saramaMsg.Headers = headers
	}

	// 发送消息
	partition, offset, err := p.producer.SendMessage(saramaMsg)
	if err != nil {
		p.logger.Error("send message failed",
			zap.String("topic", msg.Topic),
			zap.Error(err))
		return nil, fmt.Errorf("send message failed: %w", err)
	}

	p.logger.Debug("message sent",
		zap.String("topic", msg.Topic),
		zap.Int32("partition", partition),
		zap.Int64("offset", offset))

	return &ProducerResult{
		Topic:     msg.Topic,
		Partition: partition,
		Offset:    offset,
		Timestamp: time.Now(),
	}, nil
}

// SendAsync 异步发送（同步生产者模拟异步）
func (p *SyncProducer) SendAsync(msg *Message, callback func(*ProducerResult, error)) {
	go func() {
		result, err := p.Send(context.Background(), msg)
		if callback != nil {
			callback(result, err)
		}
	}()
}

// SendJSON 发送 JSON 消息
func (p *SyncProducer) SendJSON(ctx context.Context, topic string, key string, value interface{}) (*ProducerResult, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal json failed: %w", err)
	}

	msg := &Message{
		Topic:     topic,
		Key:       []byte(key),
		Value:     data,
		Partition: -1,
		Headers: map[string]string{
			"content-type": "application/json",
		},
	}

	return p.Send(ctx, msg)
}

// Close 关闭生产者
func (p *SyncProducer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true
	if err := p.producer.Close(); err != nil {
		return fmt.Errorf("close producer failed: %w", err)
	}

	p.logger.Debug("producer closed")
	return nil
}

// AsyncProducer 异步生产者实现
type AsyncProducer struct {
	producer   sarama.AsyncProducer
	config     ProducerConfig
	logger     *zap.Logger
	mu         sync.RWMutex
	closed     bool
	wg         sync.WaitGroup
	successCh  chan *ProducerResult
	errorCh    chan error
	stopCh     chan struct{}
}

// NewAsyncProducer 创建异步生产者
func NewAsyncProducer(brokers []string, cfg ProducerConfig, saramaCfg *sarama.Config, logger *zap.Logger) (*AsyncProducer, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	producer, err := sarama.NewAsyncProducer(brokers, saramaCfg)
	if err != nil {
		return nil, fmt.Errorf("create async producer failed: %w", err)
	}

	p := &AsyncProducer{
		producer:  producer,
		config:    cfg,
		logger:    logger,
		successCh: make(chan *ProducerResult, 100),
		errorCh:   make(chan error, 100),
		stopCh:    make(chan struct{}),
	}

	// 启动结果处理协程
	p.wg.Add(1)
	go p.handleResults()

	return p, nil
}

// handleResults 处理异步结果
func (p *AsyncProducer) handleResults() {
	defer p.wg.Done()

	for {
		select {
		case <-p.stopCh:
			return
		case msg := <-p.producer.Successes():
			if msg != nil {
				result := &ProducerResult{
					Topic:     msg.Topic,
					Partition: msg.Partition,
					Offset:    msg.Offset,
					Timestamp: msg.Timestamp,
				}
				select {
				case p.successCh <- result:
				default:
					p.logger.Warn("success channel full, dropping result")
				}
			}
		case err := <-p.producer.Errors():
			if err != nil {
				p.logger.Error("async send failed",
					zap.String("topic", err.Msg.Topic),
					zap.Error(err.Err))
				select {
				case p.errorCh <- err.Err:
				default:
					p.logger.Warn("error channel full, dropping error")
				}
			}
		}
	}
}

// Send 同步发送（等待结果）
func (p *AsyncProducer) Send(ctx context.Context, msg *Message) (*ProducerResult, error) {
	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		return nil, fmt.Errorf("producer is closed")
	}
	p.mu.RUnlock()

	if msg == nil {
		return nil, fmt.Errorf("message cannot be nil")
	}

	if msg.Topic == "" {
		return nil, fmt.Errorf("topic cannot be empty")
	}

	// 构建消息
	saramaMsg := &sarama.ProducerMessage{
		Topic: msg.Topic,
		Value: sarama.ByteEncoder(msg.Value),
	}

	if len(msg.Key) > 0 {
		saramaMsg.Key = sarama.ByteEncoder(msg.Key)
	}

	// 发送消息
	p.producer.Input() <- saramaMsg

	// 等待结果
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-p.successCh:
		return result, nil
	case err := <-p.errorCh:
		return nil, err
	}
}

// SendAsync 异步发送消息
func (p *AsyncProducer) SendAsync(msg *Message, callback func(*ProducerResult, error)) {
	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		if callback != nil {
			callback(nil, fmt.Errorf("producer is closed"))
		}
		return
	}
	p.mu.RUnlock()

	if msg == nil {
		if callback != nil {
			callback(nil, fmt.Errorf("message cannot be nil"))
		}
		return
	}

	saramaMsg := &sarama.ProducerMessage{
		Topic: msg.Topic,
		Value: sarama.ByteEncoder(msg.Value),
	}

	if len(msg.Key) > 0 {
		saramaMsg.Key = sarama.ByteEncoder(msg.Key)
	}

	p.producer.Input() <- saramaMsg

	// 如果有回调，启动协程等待结果
	if callback != nil {
		go func() {
			select {
			case result := <-p.successCh:
				callback(result, nil)
			case err := <-p.errorCh:
				callback(nil, err)
			}
		}()
	}
}

// SendJSON 发送 JSON 消息
func (p *AsyncProducer) SendJSON(ctx context.Context, topic string, key string, value interface{}) (*ProducerResult, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal json failed: %w", err)
	}

	msg := &Message{
		Topic:     topic,
		Key:       []byte(key),
		Value:     data,
		Partition: -1,
	}

	return p.Send(ctx, msg)
}

// Successes 返回成功通道
func (p *AsyncProducer) Successes() <-chan *ProducerResult {
	return p.successCh
}

// Errors 返回错误通道
func (p *AsyncProducer) Errors() <-chan error {
	return p.errorCh
}

// Close 关闭生产者
func (p *AsyncProducer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true
	close(p.stopCh)
	p.wg.Wait()

	if err := p.producer.Close(); err != nil {
		return fmt.Errorf("close async producer failed: %w", err)
	}

	close(p.successCh)
	close(p.errorCh)

	p.logger.Debug("async producer closed")
	return nil
}

