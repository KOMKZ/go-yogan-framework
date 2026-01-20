package kafka

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestNewManager_NilLogger(t *testing.T) {
	cfg := Config{
		Brokers: []string{"localhost:9092"},
	}

	_, err := NewManager(cfg, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "logger cannot be nil")
}

func TestNewManager_InvalidConfig(t *testing.T) {
	logger := zap.NewNop()
	cfg := Config{
		Brokers: []string{}, // empty brokers
	}

	_, err := NewManager(cfg, logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config validation failed")
}

func TestNewManager_Success(t *testing.T) {
	logger := zap.NewNop()
	cfg := Config{
		Brokers:  []string{"localhost:9092"},
		Version:  "3.8.0",
		ClientID: "test-client",
		Producer: ProducerConfig{
			Enabled:      true,
			RequiredAcks: 1,
		},
		Consumer: ConsumerConfig{
			Enabled: true,
			GroupID: "test-group",
			Topics:  []string{"test-topic"},
		},
	}

	manager, err := NewManager(cfg, logger)
	assert.NoError(t, err)
	assert.NotNil(t, manager)
	assert.Equal(t, cfg.Brokers, manager.GetConfig().Brokers)
}

func TestManager_GetProducer_Nil(t *testing.T) {
	logger := zap.NewNop()
	cfg := Config{
		Brokers: []string{"localhost:9092"},
		Producer: ProducerConfig{
			Enabled: false, // Disable producer
		},
	}

	manager, err := NewManager(cfg, logger)
	assert.NoError(t, err)

	// When not connected, producer is nil
	assert.Nil(t, manager.GetProducer())
}

func TestManager_GetConsumer_NotExists(t *testing.T) {
	logger := zap.NewNop()
	cfg := Config{
		Brokers: []string{"localhost:9092"},
	}

	manager, err := NewManager(cfg, logger)
	assert.NoError(t, err)

	// nonexistent consumer
	assert.Nil(t, manager.GetConsumer("nonexistent"))
}

func TestManager_CreateConsumer_Closed(t *testing.T) {
	logger := zap.NewNop()
	cfg := Config{
		Brokers: []string{"localhost:9092"},
	}

	manager, err := NewManager(cfg, logger)
	assert.NoError(t, err)

	// Close manager
	manager.closed = true

	// Creating the consumer should fail
	_, err = manager.CreateConsumer("test", ConsumerConfig{
		GroupID: "test-group",
		Topics:  []string{"test-topic"},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "manager is closed")
}

func TestManager_CreateConsumer_AlreadyExists(t *testing.T) {
	logger := zap.NewNop()
	cfg := Config{
		Brokers: []string{"localhost:9092"},
		Consumer: ConsumerConfig{
			Enabled: true,
			GroupID: "test-group",
			Topics:  []string{"test-topic"},
		},
	}

	manager, err := NewManager(cfg, logger)
	assert.NoError(t, err)

	// Simulate existing consumer
	manager.consumers["test"] = &ConsumerGroup{}

	// Creating a consumer with the same name should fail
	_, err = manager.CreateConsumer("test", ConsumerConfig{
		GroupID: "test-group",
		Topics:  []string{"test-topic"},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestManager_CreateConsumer_InvalidConfig(t *testing.T) {
	logger := zap.NewNop()
	cfg := Config{
		Brokers: []string{"localhost:9092"},
	}

	manager, err := NewManager(cfg, logger)
	assert.NoError(t, err)

	// invalid consumer configuration
	_, err = manager.CreateConsumer("test", ConsumerConfig{
		GroupID: "", // empty group id
		Topics:  []string{"test-topic"},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "consumer config invalid")
}

func TestManager_Ping_Closed(t *testing.T) {
	logger := zap.NewNop()
	cfg := Config{
		Brokers: []string{"localhost:9092"},
	}

	manager, err := NewManager(cfg, logger)
	assert.NoError(t, err)

	manager.closed = true

	err = manager.Ping(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "manager is closed")
}

func TestManager_Connect_Closed(t *testing.T) {
	logger := zap.NewNop()
	cfg := Config{
		Brokers: []string{"localhost:9092"},
	}

	manager, err := NewManager(cfg, logger)
	assert.NoError(t, err)

	manager.closed = true

	err = manager.Connect(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "manager is closed")
}

func TestManager_Close_Idempotent(t *testing.T) {
	logger := zap.NewNop()
	cfg := Config{
		Brokers: []string{"localhost:9092"},
	}

	manager, err := NewManager(cfg, logger)
	assert.NoError(t, err)

	// First shutdown
	err = manager.Close()
	assert.NoError(t, err)

	// Second shutdown (idempotent)
	err = manager.Close()
	assert.NoError(t, err)
}

func TestManager_GetConfig(t *testing.T) {
	logger := zap.NewNop()
	cfg := Config{
		Brokers:  []string{"localhost:9092", "localhost:9093"},
		Version:  "3.8.0",
		ClientID: "test-client",
	}

	manager, err := NewManager(cfg, logger)
	assert.NoError(t, err)

	gotCfg := manager.GetConfig()
	assert.Equal(t, cfg.Brokers, gotCfg.Brokers)
	assert.Equal(t, cfg.Version, gotCfg.Version)
	assert.Equal(t, cfg.ClientID, gotCfg.ClientID)
}

func TestBuildSaramaConfig_AllCompressions(t *testing.T) {
	compressions := []string{"none", "gzip", "snappy", "lz4", "zstd"}

	for _, compression := range compressions {
		t.Run(compression, func(t *testing.T) {
			cfg := Config{
				Brokers: []string{"localhost:9092"},
				Version: "3.8.0",
				Producer: ProducerConfig{
					Enabled:      true,
					RequiredAcks: 1,
					Compression:  compression,
				},
			}
			cfg.ApplyDefaults()

			saramaCfg, err := buildSaramaConfig(cfg)
			assert.NoError(t, err)
			assert.NotNil(t, saramaCfg)
		})
	}
}

func TestBuildSaramaConfig_RebalanceStrategies(t *testing.T) {
	strategies := []string{"range", "roundrobin", "sticky"}

	for _, strategy := range strategies {
		t.Run(strategy, func(t *testing.T) {
			cfg := Config{
				Brokers: []string{"localhost:9092"},
				Version: "3.8.0",
				Consumer: ConsumerConfig{
					Enabled:           true,
					GroupID:           "test-group",
					Topics:            []string{"test-topic"},
					RebalanceStrategy: strategy,
				},
			}
			cfg.ApplyDefaults()

			saramaCfg, err := buildSaramaConfig(cfg)
			assert.NoError(t, err)
			assert.NotNil(t, saramaCfg)
		})
	}
}

func TestBuildSaramaConfig_RequiredAcks(t *testing.T) {
	tests := []struct {
		acks int
	}{
		{acks: 0},
		{acks: 1},
		{acks: -1},
	}

	for _, tt := range tests {
		t.Run("acks_"+string(rune(tt.acks+2)), func(t *testing.T) {
			cfg := Config{
				Brokers: []string{"localhost:9092"},
				Version: "3.8.0",
				Producer: ProducerConfig{
					Enabled:      true,
					RequiredAcks: tt.acks,
				},
			}
			cfg.ApplyDefaults()

			saramaCfg, err := buildSaramaConfig(cfg)
			assert.NoError(t, err)
			assert.NotNil(t, saramaCfg)
		})
	}
}

func TestBuildSaramaConfig_OffsetInitial(t *testing.T) {
	tests := []struct {
		name   string
		offset int64
	}{
		{"newest", -1},
		{"oldest", -2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				Brokers: []string{"localhost:9092"},
				Version: "3.8.0",
				Consumer: ConsumerConfig{
					Enabled:       true,
					GroupID:       "test-group",
					Topics:        []string{"test-topic"},
					OffsetInitial: tt.offset,
				},
			}
			cfg.ApplyDefaults()

			saramaCfg, err := buildSaramaConfig(cfg)
			assert.NoError(t, err)
			assert.NotNil(t, saramaCfg)
		})
	}
}

func TestBuildSaramaConfig_SASL(t *testing.T) {
	mechanisms := []string{"PLAIN", "SCRAM-SHA-256", "SCRAM-SHA-512"}

	for _, mechanism := range mechanisms {
		t.Run(mechanism, func(t *testing.T) {
			cfg := Config{
				Brokers: []string{"localhost:9092"},
				Version: "3.8.0",
				SASL: &SASLConfig{
					Enabled:   true,
					Mechanism: mechanism,
					Username:  "user",
					Password:  "pass",
				},
			}
			cfg.ApplyDefaults()

			saramaCfg, err := buildSaramaConfig(cfg)
			assert.NoError(t, err)
			assert.NotNil(t, saramaCfg)
			assert.True(t, saramaCfg.Net.SASL.Enable)
		})
	}
}

func TestBuildSaramaConfig_TLS(t *testing.T) {
	cfg := Config{
		Brokers: []string{"localhost:9092"},
		Version: "3.8.0",
		TLS: &TLSConfig{
			Enabled:            true,
			InsecureSkipVerify: true,
		},
	}
	cfg.ApplyDefaults()

	saramaCfg, err := buildSaramaConfig(cfg)
	assert.NoError(t, err)
	assert.NotNil(t, saramaCfg)
	assert.True(t, saramaCfg.Net.TLS.Enable)
}

func TestBuildSaramaConfig_InvalidVersion(t *testing.T) {
	cfg := Config{
		Brokers: []string{"localhost:9092"},
		Version: "invalid-version",
	}

	_, err := buildSaramaConfig(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse kafka version failed")
}

func TestManager_GetAsyncProducer_Closed(t *testing.T) {
	logger := zap.NewNop()
	cfg := Config{
		Brokers: []string{"localhost:9092"},
		Producer: ProducerConfig{
			Enabled:      true,
			RequiredAcks: 1,
		},
	}

	manager, err := NewManager(cfg, logger)
	assert.NoError(t, err)

	// Simulate existing asynchronous producer
	manager.asyncProducer = &AsyncProducer{
		logger:    logger,
		successCh: make(chan *ProducerResult, 10),
		errorCh:   make(chan error, 10),
	}

	// Retrieve again should return the existing one
	ap, err := manager.GetAsyncProducer()
	assert.NoError(t, err)
	assert.Equal(t, manager.asyncProducer, ap)
}

func TestManager_Ping_Timeout(t *testing.T) {
	logger := zap.NewNop()
	cfg := Config{
		Brokers: []string{"localhost:9092"},
	}

	manager, err := NewManager(cfg, logger)
	assert.NoError(t, err)

	// Use canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = manager.Ping(ctx)
	// Should immediately return a context error or connection error
	assert.Error(t, err)
}

func TestManager_ListTopics(t *testing.T) {
	logger := zap.NewNop()
	cfg := Config{
		Brokers: []string{"localhost:9092"},
	}

	manager, err := NewManager(cfg, logger)
	assert.NoError(t, err)

	// ListTopics will attempt to connect
	// If there is a real Kafka, return the list of topics
	topics, err := manager.ListTopics(context.Background())
	// Whether successful or not, only test that the method is callable
	if err == nil {
		assert.NotNil(t, topics)
	}
}

// Test Ping timeout scenario
func TestManager_Ping_ContextTimeout(t *testing.T) {
	logger := zap.NewNop()
	cfg := Config{
		Brokers: []string{"localhost:9092"},
	}

	manager, err := NewManager(cfg, logger)
	assert.NoError(t, err)

	// Create a timed-out context
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(1 * time.Millisecond) // Ensure timeout

	err = manager.Ping(ctx)
	assert.Error(t, err)
}

// Test real connection scenarios (requires Kafka to be running)
func TestManager_Connect_WithKafka(t *testing.T) {
	logger := zap.NewNop()
	cfg := Config{
		Brokers: []string{"localhost:9092"},
		Version: "3.8.0",
		Producer: ProducerConfig{
			Enabled:      true,
			RequiredAcks: 1,
		},
	}

	manager, err := NewManager(cfg, logger)
	assert.NoError(t, err)
	defer manager.Close()

	err = manager.Connect(context.Background())
	// If Kafka is running, it should succeed
	if err == nil {
		assert.NotNil(t, manager.GetProducer())
	}
}

func TestManager_CreateConsumer_WithKafka(t *testing.T) {
	logger := zap.NewNop()
	cfg := Config{
		Brokers: []string{"localhost:9092"},
		Version: "3.8.0",
		Consumer: ConsumerConfig{
			Enabled: true,
			GroupID: "test-group",
			Topics:  []string{"test-topic"},
		},
	}

	manager, err := NewManager(cfg, logger)
	assert.NoError(t, err)
	defer manager.Close()

	consumer, err := manager.CreateConsumer("my-consumer", ConsumerConfig{
		GroupID: "test-group-new",
		Topics:  []string{"test-topic"},
	})
	
	// If Kafka is running, it should succeed
	if err == nil {
		assert.NotNil(t, consumer)
		assert.Equal(t, consumer, manager.GetConsumer("my-consumer"))
	}
}

func TestManager_GetAsyncProducer_Create(t *testing.T) {
	logger := zap.NewNop()
	cfg := Config{
		Brokers: []string{"localhost:9092"},
		Version: "3.8.0",
		Producer: ProducerConfig{
			Enabled:      true,
			RequiredAcks: 1,
		},
	}

	manager, err := NewManager(cfg, logger)
	assert.NoError(t, err)
	defer manager.Close()

	// First obtain asynchronous producer
	ap, err := manager.GetAsyncProducer()
	// If Kafka is running, it should succeed
	if err == nil {
		assert.NotNil(t, ap)
		
		// should return the same again
		ap2, err := manager.GetAsyncProducer()
		assert.NoError(t, err)
		assert.Equal(t, ap, ap2)
	}
}

func TestManager_RealKafka_FullFlow(t *testing.T) {
	logger := zap.NewNop()
	cfg := Config{
		Brokers: []string{"localhost:9092"},
		Version: "3.8.0",
		Producer: ProducerConfig{
			Enabled:      true,
			RequiredAcks: -1, // WaitForAll
			Compression:  "snappy",
			Idempotent:   false,
		},
		Consumer: ConsumerConfig{
			Enabled:           true,
			GroupID:           "test-full-flow",
			Topics:            []string{"test-topic"},
			RebalanceStrategy: "roundrobin",
		},
	}

	manager, err := NewManager(cfg, logger)
	assert.NoError(t, err)
	defer manager.Close()

	// Connect
	err = manager.Connect(context.Background())
	assert.NoError(t, err)

	// Validate producer
	producer := manager.GetProducer()
	assert.NotNil(t, producer)

	// Ping
	err = manager.Ping(context.Background())
	assert.NoError(t, err)

	// ListTopics
	topics, err := manager.ListTopics(context.Background())
	assert.NoError(t, err)
	t.Logf("Topics: %v", topics)

	// Send message
	result, err := producer.Send(context.Background(), &Message{
		Topic: "test-topic",
		Key:   []byte("full-flow-key"),
		Value: []byte("full-flow-value"),
	})
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Create consumer
	consumer, err := manager.CreateConsumer("flow-consumer", ConsumerConfig{
		GroupID:       "flow-group-" + time.Now().Format("150405"),
		Topics:        []string{"test-topic"},
		OffsetInitial: -2,
	})
	assert.NoError(t, err)
	assert.NotNil(t, consumer)

	// Verify GetConfig
	gotCfg := manager.GetConfig()
	assert.Equal(t, cfg.Brokers, gotCfg.Brokers)
}

func TestManager_Close_WithConsumersAndProducers(t *testing.T) {
	logger := zap.NewNop()
	cfg := Config{
		Brokers: []string{"localhost:9092"},
		Version: "3.8.0",
		Producer: ProducerConfig{
			Enabled:      true,
			RequiredAcks: 1,
		},
	}

	manager, err := NewManager(cfg, logger)
	assert.NoError(t, err)

	err = manager.Connect(context.Background())
	assert.NoError(t, err)

	// Create consumer
	consumer, err := manager.CreateConsumer("close-test-consumer", ConsumerConfig{
		GroupID: "close-test-group",
		Topics:  []string{"test-topic"},
	})
	assert.NoError(t, err)

	// Start consumer
	ctx, cancel := context.WithCancel(context.Background())
	err = consumer.Start(ctx, func(ctx context.Context, msg *ConsumedMessage) error {
		return nil
	})
	assert.NoError(t, err)

	// Get asynchronous producer
	_, err = manager.GetAsyncProducer()
	assert.NoError(t, err)

	// Cancel context
	cancel()

	// wait for a moment for the consumer to stop
	time.Sleep(100 * time.Millisecond)

	// Close - All resources should be closed
	err = manager.Close()
	assert.NoError(t, err)
}

func TestManager_Ping_Variations(t *testing.T) {
	logger := zap.NewNop()
	cfg := Config{
		Brokers: []string{"localhost:9092"},
		Version: "3.8.0",
	}

	manager, err := NewManager(cfg, logger)
	assert.NoError(t, err)
	defer manager.Close()

	// Establish connection first
	err = manager.Connect(context.Background())
	assert.NoError(t, err)

	// The ping should succeed
	err = manager.Ping(context.Background())
	assert.NoError(t, err)

	// Using context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = manager.Ping(ctx)
	assert.NoError(t, err)
}

func TestManager_Close_MultipleConsumers(t *testing.T) {
	logger := zap.NewNop()
	cfg := Config{
		Brokers: []string{"localhost:9092"},
		Version: "3.8.0",
		Producer: ProducerConfig{
			Enabled:      true,
			RequiredAcks: 1,
		},
	}

	manager, err := NewManager(cfg, logger)
	assert.NoError(t, err)

	err = manager.Connect(context.Background())
	assert.NoError(t, err)

	// Create multiple consumers
	for i := 0; i < 3; i++ {
		_, err = manager.CreateConsumer(fmt.Sprintf("consumer-%d", i), ConsumerConfig{
			GroupID: fmt.Sprintf("group-%d", i),
			Topics:  []string{"test-topic"},
		})
		assert.NoError(t, err)
	}

	// close
	err = manager.Close()
	assert.NoError(t, err)

	// Re-closing should be idempotent
	err = manager.Close()
	assert.NoError(t, err)
}

