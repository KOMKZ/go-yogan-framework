package kafka

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConsumerHandlerFunc_Name(t *testing.T) {
	handler := NewConsumerHandlerFunc("test-consumer", []string{"topic1"}, nil)
	assert.Equal(t, "test-consumer", handler.Name())
}

func TestConsumerHandlerFunc_Topics(t *testing.T) {
	topics := []string{"topic1", "topic2"}
	handler := NewConsumerHandlerFunc("test", topics, nil)
	assert.Equal(t, topics, handler.Topics())
}

func TestConsumerHandlerFunc_Handle_Success(t *testing.T) {
	called := false
	handler := NewConsumerHandlerFunc("test", []string{"topic"}, func(ctx context.Context, msg *ConsumedMessage) error {
		called = true
		assert.Equal(t, "test-topic", msg.Topic)
		assert.Equal(t, []byte("test-value"), msg.Value)
		return nil
	})

	msg := &ConsumedMessage{
		Topic: "test-topic",
		Value: []byte("test-value"),
	}

	err := handler.Handle(context.Background(), msg)
	assert.NoError(t, err)
	assert.True(t, called)
}

func TestConsumerHandlerFunc_Handle_Error(t *testing.T) {
	expectedErr := errors.New("handle error")
	handler := NewConsumerHandlerFunc("test", []string{"topic"}, func(ctx context.Context, msg *ConsumedMessage) error {
		return expectedErr
	})

	err := handler.Handle(context.Background(), &ConsumedMessage{})
	assert.ErrorIs(t, err, expectedErr)
}

func TestConsumerHandlerFunc_Handle_NilHandler(t *testing.T) {
	handler := NewConsumerHandlerFunc("test", []string{"topic"}, nil)
	err := handler.Handle(context.Background(), &ConsumedMessage{})
	assert.NoError(t, err)
}

func TestConsumerHandlerFunc_ImplementsInterface(t *testing.T) {
	var _ ConsumerHandler = (*ConsumerHandlerFunc)(nil)
}
