package kafka

import (
	"context"
	"encoding/json"
	"fmt"
)

// PublishMessage Convenient structure for publishing messages
type PublishMessage struct {
	// Key message key (optional)
	Key string

	// Message value
	Value []byte

	// Headers (optional)
	Headers map[string]string
}

// Publish message to specified topic
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

// PublishJSON publishes JSON messages to a specified topic
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

// PublishBytes publish byte message to specified Topic
func (m *Manager) PublishBytes(ctx context.Context, topic string, key string, value []byte) error {
	return m.Publish(ctx, topic, &PublishMessage{
		Key:   key,
		Value: value,
	})
}

// PublishString Publish string message to specified Topic
func (m *Manager) PublishString(ctx context.Context, topic string, key string, value string) error {
	return m.Publish(ctx, topic, &PublishMessage{
		Key:   key,
		Value: []byte(value),
	})
}

// PublishWithHeaders publish messages with headers
func (m *Manager) PublishWithHeaders(ctx context.Context, topic string, key string, value []byte, headers map[string]string) error {
	return m.Publish(ctx, topic, &PublishMessage{
		Key:     key,
		Value:   value,
		Headers: headers,
	})
}
