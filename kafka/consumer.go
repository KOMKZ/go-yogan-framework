package kafka

import (
	"context"
	"fmt"
	"sync"

	"github.com/IBM/sarama"
	"github.com/KOMKZ/go-yogan-framework/logger"
	"go.uber.org/zap"
)

// Consumed message
type ConsumedMessage struct {
	// Topic message source Topic
	Topic string

	// Partition message partitioning
	Partition int32

	// Message Offset
	Offset int64

	// Key message key
	Key []byte

	// Message value
	Value []byte

	// Headers header information
	Headers map[string]string

	// Timestamp Message timestamp
	Timestamp int64
}

// MessageHandler message processing function
type MessageHandler func(ctx context.Context, msg *ConsumedMessage) error

// Kafka consumer interface
type Consumer interface {
	// Start the consumer
	Start(ctx context.Context, handler MessageHandler) error

	// Stop Terminate consumer
	Stop() error

	// Check if running
	IsRunning() bool
}

// ConsumerGroup implementation
type ConsumerGroup struct {
	group    sarama.ConsumerGroup
	config   ConsumerConfig
	brokers  []string
	logger   *logger.CtxZapLogger
	handler  MessageHandler
	mu       sync.RWMutex
	running  bool
	stopCh   chan struct{}
	doneCh   chan struct{}
}

// Create consumer group NewConsumerGroup
func NewConsumerGroup(brokers []string, cfg ConsumerConfig, saramaCfg *sarama.Config, log *logger.CtxZapLogger) (*ConsumerGroup, error) {
	if log == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	if cfg.GroupID == "" {
		return nil, fmt.Errorf("group_id cannot be empty")
	}

	group, err := sarama.NewConsumerGroup(brokers, cfg.GroupID, saramaCfg)
	if err != nil {
		return nil, fmt.Errorf("create consumer group failed: %w", err)
	}

	return &ConsumerGroup{
		group:   group,
		config:  cfg,
		brokers: brokers,
		logger:  log,
		stopCh:  make(chan struct{}),
		doneCh:  make(chan struct{}),
	}, nil
}

// Start Consumer Initialization
func (c *ConsumerGroup) Start(ctx context.Context, handler MessageHandler) error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return fmt.Errorf("consumer is already running")
	}
	c.running = true
	c.handler = handler
	c.mu.Unlock()

	go c.consumeLoop(ctx)

	c.logger.InfoCtx(ctx, "consumer group started",
		zap.String("group_id", c.config.GroupID),
		zap.Strings("topics", c.config.Topics))

	return nil
}

// consumeLoop consumption loop
func (c *ConsumerGroup) consumeLoop(ctx context.Context) {
	defer close(c.doneCh)

	consumerHandler := &consumerGroupHandler{
		handler: c.handler,
		logger:  c.logger,
	}

	for {
		select {
		case <-c.stopCh:
			c.logger.DebugCtx(ctx, "consumer loop stopped by signal")
			return
		case <-ctx.Done():
			c.logger.DebugCtx(ctx, "consumer loop stopped by context")
			return
		default:
			// Consume message
			err := c.group.Consume(ctx, c.config.Topics, consumerHandler)
			if err != nil {
				c.logger.ErrorCtx(ctx, "consume error", zap.Error(err))
			}

			// Check if context is cancelled
			if ctx.Err() != nil {
				return
			}
		}
	}
}

// Stop Terminate consumer
func (c *ConsumerGroup) Stop() error {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return nil
	}
	c.running = false
	c.mu.Unlock()

	close(c.stopCh)

	// wait for consumption loop to end
	<-c.doneCh

	if err := c.group.Close(); err != nil {
		return fmt.Errorf("close consumer group failed: %w", err)
	}

	c.logger.InfoCtx(context.Background(), "consumer group stopped",
		zap.String("group_id", c.config.GroupID))

	return nil
}

// Check if running
func (c *ConsumerGroup) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.running
}

// consumerGroupHandler implements the sarama.ConsumerGroupHandler interface
type consumerGroupHandler struct {
	handler MessageHandler
	logger  *logger.CtxZapLogger
}

// Setup called at the start of a new session
func (h *consumerGroupHandler) Setup(session sarama.ConsumerGroupSession) error {
	h.logger.DebugCtx(session.Context(), "consumer session setup",
		zap.Int32("generation_id", session.GenerationID()),
		zap.String("member_id", session.MemberID()))
	return nil
}

// Cleanup is called at the end of the session
func (h *consumerGroupHandler) Cleanup(session sarama.ConsumerGroupSession) error {
	h.logger.DebugCtx(session.Context(), "consumer session cleanup",
		zap.Int32("generation_id", session.GenerationID()))
	return nil
}

// Consume partition messages
func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case <-session.Context().Done():
			return nil
		case msg, ok := <-claim.Messages():
			if !ok {
				return nil
			}

			// Build consumption message
			consumedMsg := &ConsumedMessage{
				Topic:     msg.Topic,
				Partition: msg.Partition,
				Offset:    msg.Offset,
				Key:       msg.Key,
				Value:     msg.Value,
				Timestamp: msg.Timestamp.UnixMilli(),
				Headers:   make(map[string]string),
			}

			// Parse Headers
			for _, header := range msg.Headers {
				consumedMsg.Headers[string(header.Key)] = string(header.Value)
			}

			// Call processing function
			if h.handler != nil {
				if err := h.handler(session.Context(), consumedMsg); err != nil {
					h.logger.ErrorCtx(session.Context(), "handle message failed",
						zap.String("topic", msg.Topic),
						zap.Int32("partition", msg.Partition),
						zap.Int64("offset", msg.Offset),
						zap.Error(err))
					// Proceed to process the next message without interruption
				}
			}

			// Mark message as processed
			session.MarkMessage(msg, "")
		}
	}
}

// SimpleConsumer (single Topic, single partition)
type SimpleConsumer struct {
	consumer sarama.Consumer
	config   ConsumerConfig
	logger   *logger.CtxZapLogger
	mu       sync.RWMutex
	running  bool
	stopCh   chan struct{}
}

// Create simple consumer
func NewSimpleConsumer(brokers []string, saramaCfg *sarama.Config, log *logger.CtxZapLogger) (*SimpleConsumer, error) {
	if log == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	consumer, err := sarama.NewConsumer(brokers, saramaCfg)
	if err != nil {
		return nil, fmt.Errorf("create consumer failed: %w", err)
	}

	return &SimpleConsumer{
		consumer: consumer,
		logger:   log,
		stopCh:   make(chan struct{}),
	}, nil
}

// ConsumePartition Consume the specified partition
func (c *SimpleConsumer) ConsumePartition(ctx context.Context, topic string, partition int32, offset int64, handler MessageHandler) error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return fmt.Errorf("consumer is already running")
	}
	c.running = true
	c.mu.Unlock()

	partitionConsumer, err := c.consumer.ConsumePartition(topic, partition, offset)
	if err != nil {
		return fmt.Errorf("consume partition failed: %w", err)
	}

	go func() {
		defer partitionConsumer.Close()

		for {
			select {
			case <-c.stopCh:
				return
			case <-ctx.Done():
				return
			case msg := <-partitionConsumer.Messages():
				if msg == nil {
					continue
				}

				consumedMsg := &ConsumedMessage{
					Topic:     msg.Topic,
					Partition: msg.Partition,
					Offset:    msg.Offset,
					Key:       msg.Key,
					Value:     msg.Value,
					Timestamp: msg.Timestamp.UnixMilli(),
					Headers:   make(map[string]string),
				}

				for _, header := range msg.Headers {
					consumedMsg.Headers[string(header.Key)] = string(header.Value)
				}

				if handler != nil {
					if err := handler(ctx, consumedMsg); err != nil {
						c.logger.ErrorCtx(ctx, "handle message failed", zap.Error(err))
					}
				}
			case err := <-partitionConsumer.Errors():
				if err != nil {
					c.logger.ErrorCtx(ctx, "partition consumer error", zap.Error(err))
				}
			}
		}
	}()

	return nil
}

// Stop Terminate consumer
func (c *SimpleConsumer) Stop() error {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return nil
	}
	c.running = false
	c.mu.Unlock()

	close(c.stopCh)
	return c.consumer.Close()
}

// Check if running
func (c *SimpleConsumer) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.running
}

