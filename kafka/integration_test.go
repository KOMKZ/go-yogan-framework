// +build integration

package kafka

import (
	"context"
	"testing"
	"time"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/stretchr/testify/assert"
)

// Integration tests require a real Kafka environment
// Run: go test -tags=integration -v ./...

func getTestConfig() Config {
	return Config{
		Brokers:  []string{"localhost:19092"},
		Version:  "3.8.0",
		ClientID: "test-client",
		Producer: ProducerConfig{
			Enabled:      true,
			RequiredAcks: 1,
			Timeout:      10 * time.Second,
		},
		Consumer: ConsumerConfig{
			Enabled:       true,
			GroupID:       "test-group",
			Topics:        []string{"test-topic"},
			OffsetInitial: -2, // Oldest
			AutoCommit:    true,
		},
	}
}

func TestIntegration_Manager_Connect(t *testing.T) {
	log := logger.GetLogger("test")
	cfg := getTestConfig()

	manager, err := NewManager(cfg, log)
	assert.NoError(t, err)
	defer manager.Close()

	err = manager.Connect(context.Background())
	assert.NoError(t, err)

	// Verify that the producer has been created
	assert.NotNil(t, manager.GetProducer())
}

func TestIntegration_Manager_Ping(t *testing.T) {
	log := logger.GetLogger("test")
	cfg := getTestConfig()

	manager, err := NewManager(cfg, log)
	assert.NoError(t, err)
	defer manager.Close()

	err = manager.Connect(context.Background())
	assert.NoError(t, err)

	err = manager.Ping(context.Background())
	assert.NoError(t, err)
}

func TestIntegration_Manager_ListTopics(t *testing.T) {
	log := logger.GetLogger("test")
	cfg := getTestConfig()

	manager, err := NewManager(cfg, log)
	assert.NoError(t, err)
	defer manager.Close()

	err = manager.Connect(context.Background())
	assert.NoError(t, err)

	topics, err := manager.ListTopics(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, topics)
}

func TestIntegration_Producer_Send(t *testing.T) {
	log := logger.GetLogger("test")
	cfg := getTestConfig()

	manager, err := NewManager(cfg, log)
	assert.NoError(t, err)
	defer manager.Close()

	err = manager.Connect(context.Background())
	assert.NoError(t, err)

	producer := manager.GetProducer()
	assert.NotNil(t, producer)

	msg := &Message{
		Topic: "test-topic",
		Key:   []byte("test-key"),
		Value: []byte("test-value"),
	}

	result, err := producer.Send(context.Background(), msg)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "test-topic", result.Topic)
}

func TestIntegration_Producer_SendJSON(t *testing.T) {
	log := logger.GetLogger("test")
	cfg := getTestConfig()

	manager, err := NewManager(cfg, log)
	assert.NoError(t, err)
	defer manager.Close()

	err = manager.Connect(context.Background())
	assert.NoError(t, err)

	producer := manager.GetProducer()

	data := map[string]interface{}{
		"name": "test",
		"age":  25,
	}

	result, err := producer.SendJSON(context.Background(), "test-topic", "json-key", data)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestIntegration_Consumer_Create(t *testing.T) {
	log := logger.GetLogger("test")
	cfg := getTestConfig()

	manager, err := NewManager(cfg, log)
	assert.NoError(t, err)
	defer manager.Close()

	consumer, err := manager.CreateConsumer("test-consumer", ConsumerConfig{
		GroupID:       "test-group-2",
		Topics:        []string{"test-topic"},
		OffsetInitial: -2,
		AutoCommit:    true,
	})
	assert.NoError(t, err)
	assert.NotNil(t, consumer)

	// Verify that the consumer can be retrieved
	assert.Equal(t, consumer, manager.GetConsumer("test-consumer"))
}

