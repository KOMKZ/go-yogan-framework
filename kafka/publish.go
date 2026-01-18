package kafka

import (
	"context"
	"encoding/json"
	"fmt"
)

// PublishMessage 发布消息的便捷结构
type PublishMessage struct {
	// Key 消息键（可选）
	Key string

	// Value 消息值
	Value []byte

	// Headers 消息头（可选）
	Headers map[string]string
}

// Publish 发布消息到指定 Topic
func (m *Manager) Publish(ctx context.Context, topic string, msg *PublishMessage) error {
	producer := m.GetProducer()
	if producer == nil {
		return fmt.Errorf("producer not available")
	}

	kafkaMsg := &Message{
		Topic:   topic,
		Key:     []byte(msg.Key),
		Value:   msg.Value,
		Headers: msg.Headers,
	}

	_, err := producer.Send(ctx, kafkaMsg)
	return err
}

// PublishJSON 发布 JSON 消息到指定 Topic
func (m *Manager) PublishJSON(ctx context.Context, topic string, key string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload failed: %w", err)
	}

	return m.Publish(ctx, topic, &PublishMessage{
		Key:   key,
		Value: data,
	})
}

// PublishBytes 发布字节消息到指定 Topic
func (m *Manager) PublishBytes(ctx context.Context, topic string, key string, value []byte) error {
	return m.Publish(ctx, topic, &PublishMessage{
		Key:   key,
		Value: value,
	})
}

// PublishString 发布字符串消息到指定 Topic
func (m *Manager) PublishString(ctx context.Context, topic string, key string, value string) error {
	return m.Publish(ctx, topic, &PublishMessage{
		Key:   key,
		Value: []byte(value),
	})
}

// PublishWithHeaders 发布带 Headers 的消息
func (m *Manager) PublishWithHeaders(ctx context.Context, topic string, key string, value []byte, headers map[string]string) error {
	return m.Publish(ctx, topic, &PublishMessage{
		Key:     key,
		Value:   value,
		Headers: headers,
	})
}
