package kafka

import (
	"context"
	"fmt"
	"sync"

	"github.com/IBM/sarama"
	"go.uber.org/zap"
)

// ConsumedMessage 消费的消息
type ConsumedMessage struct {
	// Topic 消息来源 Topic
	Topic string

	// Partition 消息分区
	Partition int32

	// Offset 消息 Offset
	Offset int64

	// Key 消息键
	Key []byte

	// Value 消息值
	Value []byte

	// Headers 消息头
	Headers map[string]string

	// Timestamp 消息时间戳
	Timestamp int64
}

// MessageHandler 消息处理函数
type MessageHandler func(ctx context.Context, msg *ConsumedMessage) error

// Consumer Kafka 消费者接口
type Consumer interface {
	// Start 启动消费者
	Start(ctx context.Context, handler MessageHandler) error

	// Stop 停止消费者
	Stop() error

	// IsRunning 检查是否运行中
	IsRunning() bool
}

// ConsumerGroup 消费者组实现
type ConsumerGroup struct {
	group    sarama.ConsumerGroup
	config   ConsumerConfig
	brokers  []string
	logger   *zap.Logger
	handler  MessageHandler
	mu       sync.RWMutex
	running  bool
	stopCh   chan struct{}
	doneCh   chan struct{}
}

// NewConsumerGroup 创建消费者组
func NewConsumerGroup(brokers []string, cfg ConsumerConfig, saramaCfg *sarama.Config, logger *zap.Logger) (*ConsumerGroup, error) {
	if logger == nil {
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
		logger:  logger,
		stopCh:  make(chan struct{}),
		doneCh:  make(chan struct{}),
	}, nil
}

// Start 启动消费者
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

	c.logger.Info("consumer group started",
		zap.String("group_id", c.config.GroupID),
		zap.Strings("topics", c.config.Topics))

	return nil
}

// consumeLoop 消费循环
func (c *ConsumerGroup) consumeLoop(ctx context.Context) {
	defer close(c.doneCh)

	consumerHandler := &consumerGroupHandler{
		handler: c.handler,
		logger:  c.logger,
	}

	for {
		select {
		case <-c.stopCh:
			c.logger.Debug("consumer loop stopped by signal")
			return
		case <-ctx.Done():
			c.logger.Debug("consumer loop stopped by context")
			return
		default:
			// 消费消息
			err := c.group.Consume(ctx, c.config.Topics, consumerHandler)
			if err != nil {
				c.logger.Error("consume error", zap.Error(err))
			}

			// 检查 context 是否取消
			if ctx.Err() != nil {
				return
			}
		}
	}
}

// Stop 停止消费者
func (c *ConsumerGroup) Stop() error {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return nil
	}
	c.running = false
	c.mu.Unlock()

	close(c.stopCh)

	// 等待消费循环结束
	<-c.doneCh

	if err := c.group.Close(); err != nil {
		return fmt.Errorf("close consumer group failed: %w", err)
	}

	c.logger.Info("consumer group stopped",
		zap.String("group_id", c.config.GroupID))

	return nil
}

// IsRunning 检查是否运行中
func (c *ConsumerGroup) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.running
}

// consumerGroupHandler 实现 sarama.ConsumerGroupHandler 接口
type consumerGroupHandler struct {
	handler MessageHandler
	logger  *zap.Logger
}

// Setup 在新会话开始时调用
func (h *consumerGroupHandler) Setup(session sarama.ConsumerGroupSession) error {
	h.logger.Debug("consumer session setup",
		zap.Int32("generation_id", session.GenerationID()),
		zap.String("member_id", session.MemberID()))
	return nil
}

// Cleanup 在会话结束时调用
func (h *consumerGroupHandler) Cleanup(session sarama.ConsumerGroupSession) error {
	h.logger.Debug("consumer session cleanup",
		zap.Int32("generation_id", session.GenerationID()))
	return nil
}

// ConsumeClaim 消费分区消息
func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case <-session.Context().Done():
			return nil
		case msg, ok := <-claim.Messages():
			if !ok {
				return nil
			}

			// 构建消费消息
			consumedMsg := &ConsumedMessage{
				Topic:     msg.Topic,
				Partition: msg.Partition,
				Offset:    msg.Offset,
				Key:       msg.Key,
				Value:     msg.Value,
				Timestamp: msg.Timestamp.UnixMilli(),
				Headers:   make(map[string]string),
			}

			// 解析 Headers
			for _, header := range msg.Headers {
				consumedMsg.Headers[string(header.Key)] = string(header.Value)
			}

			// 调用处理函数
			if h.handler != nil {
				if err := h.handler(session.Context(), consumedMsg); err != nil {
					h.logger.Error("handle message failed",
						zap.String("topic", msg.Topic),
						zap.Int32("partition", msg.Partition),
						zap.Int64("offset", msg.Offset),
						zap.Error(err))
					// 继续处理下一条消息，不中断消费
				}
			}

			// 标记消息已处理
			session.MarkMessage(msg, "")
		}
	}
}

// SimpleConsumer 简单消费者（单 Topic，单分区）
type SimpleConsumer struct {
	consumer sarama.Consumer
	config   ConsumerConfig
	logger   *zap.Logger
	mu       sync.RWMutex
	running  bool
	stopCh   chan struct{}
}

// NewSimpleConsumer 创建简单消费者
func NewSimpleConsumer(brokers []string, saramaCfg *sarama.Config, logger *zap.Logger) (*SimpleConsumer, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	consumer, err := sarama.NewConsumer(brokers, saramaCfg)
	if err != nil {
		return nil, fmt.Errorf("create consumer failed: %w", err)
	}

	return &SimpleConsumer{
		consumer: consumer,
		logger:   logger,
		stopCh:   make(chan struct{}),
	}, nil
}

// ConsumePartition 消费指定分区
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
						c.logger.Error("handle message failed", zap.Error(err))
					}
				}
			case err := <-partitionConsumer.Errors():
				if err != nil {
					c.logger.Error("partition consumer error", zap.Error(err))
				}
			}
		}
	}()

	return nil
}

// Stop 停止消费者
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

// IsRunning 检查是否运行中
func (c *SimpleConsumer) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.running
}

