package kafka

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPublishMessage_Basic(t *testing.T) {
	msg := &PublishMessage{
		Key:   "test-key",
		Value: []byte("test-value"),
		Headers: map[string]string{
			"header1": "value1",
		},
	}

	assert.Equal(t, "test-key", msg.Key)
	assert.Equal(t, []byte("test-value"), msg.Value)
	assert.Equal(t, "value1", msg.Headers["header1"])
}

func TestPublishJSON_Serialization(t *testing.T) {
	type testPayload struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	payload := testPayload{Name: "test", Value: 42}
	data, err := json.Marshal(payload)
	assert.NoError(t, err)

	var decoded testPayload
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, payload, decoded)
}

// 注意：完整的 Publish 测试需要 mock Manager 和 Producer
// 这里测试辅助函数和数据结构

func TestPublishMessage_EmptyHeaders(t *testing.T) {
	msg := &PublishMessage{
		Key:   "key",
		Value: []byte("value"),
	}

	assert.Nil(t, msg.Headers)
}

func TestPublishMessage_WithContext(t *testing.T) {
	// 验证 context 可以正常传递
	ctx := context.WithValue(context.Background(), "trace_id", "123")
	assert.Equal(t, "123", ctx.Value("trace_id"))
}

// MockProducer 用于测试
type mockProducerForPublish struct {
	sentMessages []*Message
}

func (p *mockProducerForPublish) SendMessage(msg *Message) error {
	p.sentMessages = append(p.sentMessages, msg)
	return nil
}

func (p *mockProducerForPublish) SendMessages(msgs []*Message) error {
	p.sentMessages = append(p.sentMessages, msgs...)
	return nil
}

func (p *mockProducerForPublish) Close() error {
	return nil
}

func TestMockProducer_SendMessage(t *testing.T) {
	producer := &mockProducerForPublish{}

	msg := &Message{
		Topic: "test-topic",
		Key:   []byte("key"),
		Value: []byte("value"),
	}

	err := producer.SendMessage(msg)
	assert.NoError(t, err)
	assert.Len(t, producer.sentMessages, 1)
	assert.Equal(t, "test-topic", producer.sentMessages[0].Topic)
}
