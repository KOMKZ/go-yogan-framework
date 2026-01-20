package kafka

import (
	"context"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

// testConfigLoader configuration loader for testing
type testConfigLoader struct {
	data map[string]interface{}
}

func newTestConfigLoader() *testConfigLoader {
	return &testConfigLoader{
		data: make(map[string]interface{}),
	}
}

func (m *testConfigLoader) set(key string, value interface{}) {
	m.data[key] = value
}

func (m *testConfigLoader) GetStringSlice(key string) []string {
	if v, ok := m.data[key].([]string); ok {
		return v
	}
	return nil
}

func (m *testConfigLoader) GetString(key string) string {
	if v, ok := m.data[key].(string); ok {
		return v
	}
	return ""
}

func (m *testConfigLoader) GetInt(key string) int {
	if v, ok := m.data[key].(int); ok {
		return v
	}
	return 0
}

func (m *testConfigLoader) GetInt64(key string) int64 {
	if v, ok := m.data[key].(int64); ok {
		return v
	}
	return 0
}

func (m *testConfigLoader) GetBool(key string) bool {
	if v, ok := m.data[key].(bool); ok {
		return v
	}
	return false
}

func (m *testConfigLoader) GetDuration(key string) time.Duration {
	if v, ok := m.data[key].(time.Duration); ok {
		return v
	}
	return 0
}

func (m *testConfigLoader) IsSet(key string) bool {
	_, ok := m.data[key]
	return ok
}

func (m *testConfigLoader) Sub(key string) *viper.Viper {
	return nil
}

func TestLoadConsumerRunnerConfig_Empty(t *testing.T) {
	loader := newTestConfigLoader()
	cfg := LoadConsumerRunnerConfig(loader, "test")

	// An empty configuration should return zero values
	assert.Equal(t, "", cfg.GroupID)
	assert.Equal(t, 0, cfg.Workers)
	assert.True(t, cfg.AutoCommit) // Default true
}

func TestLoadConsumerRunnerConfig_Full(t *testing.T) {
	loader := newTestConfigLoader()
	loader.set("kafka.consumers.demo.group_id", "custom-group")
	loader.set("kafka.consumers.demo.workers", 5)
	loader.set("kafka.consumers.demo.offset_initial", int64(-2))
	loader.set("kafka.consumers.demo.auto_commit", true)
	loader.set("kafka.consumers.demo.auto_commit_interval", 2*time.Second)
	loader.set("kafka.consumers.demo.max_processing_time", 30*time.Second)

	cfg := LoadConsumerRunnerConfig(loader, "demo")

	assert.Equal(t, "custom-group", cfg.GroupID)
	assert.Equal(t, 5, cfg.Workers)
	assert.Equal(t, int64(-2), cfg.OffsetInitial)
	assert.True(t, cfg.AutoCommit)
	assert.Equal(t, 2*time.Second, cfg.AutoCommitInterval)
	assert.Equal(t, 30*time.Second, cfg.MaxProcessingTime)
}

func TestLoadConsumerTopics_FromConfig(t *testing.T) {
	loader := newTestConfigLoader()
	loader.set("kafka.consumers.test.topics", []string{"topic1", "topic2"})

	topics := LoadConsumerTopics(loader, "test")
	assert.Equal(t, []string{"topic1", "topic2"}, topics)
}

func TestLoadConsumerTopics_NotSet(t *testing.T) {
	loader := newTestConfigLoader()
	topics := LoadConsumerTopics(loader, "test")
	assert.Nil(t, topics)
}

func TestMergeConfigWithHandler(t *testing.T) {
	handler := &mockHandler{
		name:   "demo",
		topics: []string{"handler-topic"},
	}

	// Scene 1: Configuration contains topics
	loader := newTestConfigLoader()
	loader.set("kafka.consumers.demo.topics", []string{"config-topic"})
	loader.set("kafka.consumers.demo.workers", 3)

	topics, cfg := MergeConfigWithHandler(handler, loader)
	assert.Equal(t, []string{"config-topic"}, topics) // configuration priority
	assert.Equal(t, 3, cfg.Workers)

	// Scenario 2: No topics specified in configuration
	loader2 := newTestConfigLoader()
	loader2.set("kafka.consumers.demo.workers", 2)

	topics2, cfg2 := MergeConfigWithHandler(handler, loader2)
	assert.Equal(t, []string{"handler-topic"}, topics2) // Handler fallback safeguard measures
	assert.Equal(t, 2, cfg2.Workers)
}

func TestConsumerConfigOverride(t *testing.T) {
	handler := &mockHandler{
		name:   "original",
		topics: []string{"original-topic"},
	}

	override := NewConsumerConfigOverride(handler, []string{"override-topic"})

	assert.Equal(t, "original", override.Name())
	assert.Equal(t, []string{"override-topic"}, override.Topics())

	// Test Proxy Handler
	err := override.Handle(context.Background(), &ConsumedMessage{})
	assert.NoError(t, err)
}

func TestConsumerConfigOverride_EmptyTopics(t *testing.T) {
	handler := &mockHandler{
		name:   "test",
		topics: []string{"handler-topic"},
	}

	// Empty topics should rollback to handler
	override := NewConsumerConfigOverride(handler, nil)
	assert.Equal(t, []string{"handler-topic"}, override.Topics())

	override2 := NewConsumerConfigOverride(handler, []string{})
	assert.Equal(t, []string{"handler-topic"}, override2.Topics())
}
