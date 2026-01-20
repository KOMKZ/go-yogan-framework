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

// Message structure
type Message struct {
	// Topic Objective Topic
	Topic string

	// Key message key (for partitioning)
	Key []byte

	// Message value
	Value []byte

	// Headers header information
	Headers map[string]string

	// Partition specifies the partition (-1 indicates automatic allocation)
	Partition int32

	// Timestamp Message timestamp
	Timestamp time.Time
}

// ProducerResult send result
type ProducerResult struct {
	// Topic sent Topic
	Topic string

	// Partition sent partition
	Partition int32

	// Message Offset
	Offset int64

	// Timestamp server timestamp
	Timestamp time.Time
}

// Kafka producer interface
type Producer interface {
	// Send synchronous message
	Send(ctx context.Context, msg *Message) (*ProducerResult, error)

	// SendAsync asynchronously send message
	SendAsync(msg *Message, callback func(*ProducerResult, error))

	// SendJSON sends JSON message
	SendJSON(ctx context.Context, topic string, key string, value interface{}) (*ProducerResult, error)

	// Close shutdown producer
	Close() error
}

// SyncProducer synchronous producer implementation
type SyncProducer struct {
	producer sarama.SyncProducer
	config   ProducerConfig
	logger   *zap.Logger
	mu       sync.RWMutex
	closed   bool
}

// NewSyncProducer creates a synchronous producer
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

// Send synchronous message
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

	// Build Sarama message
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

	// Add Headers
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

	// Send message
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

// SendAsync asynchronously sends (simulates asynchronous producer with a synchronous producer)
func (p *SyncProducer) SendAsync(msg *Message, callback func(*ProducerResult, error)) {
	go func() {
		result, err := p.Send(context.Background(), msg)
		if callback != nil {
			callback(result, err)
		}
	}()
}

// SendJSON sends JSON message
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

// Close shutdown producer
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

// AsyncProducer asynchronous producer implementation
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

// Create asynchronous producer
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

	// Start result handling coroutine
	p.wg.Add(1)
	go p.handleResults()

	return p, nil
}

// handleResults handle asynchronous results
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

// Send synchronously (wait for result)
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

	// Build message
	saramaMsg := &sarama.ProducerMessage{
		Topic: msg.Topic,
		Value: sarama.ByteEncoder(msg.Value),
	}

	if len(msg.Key) > 0 {
		saramaMsg.Key = sarama.ByteEncoder(msg.Key)
	}

	// Send message
	p.producer.Input() <- saramaMsg

	// wait for result
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-p.successCh:
		return result, nil
	case err := <-p.errorCh:
		return nil, err
	}
}

// SendAsync asynchronously send message
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

	// If there is a callback, start a coroutine to wait for the result
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

// SendJSON send JSON message
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

// Successes return successful channels
func (p *AsyncProducer) Successes() <-chan *ProducerResult {
	return p.successCh
}

// Errors return error channel
func (p *AsyncProducer) Errors() <-chan error {
	return p.errorCh
}

// Close the producer
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

