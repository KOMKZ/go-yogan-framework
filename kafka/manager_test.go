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
		Brokers: []string{}, // 空 brokers
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
			Enabled: false, // 禁用生产者
		},
	}

	manager, err := NewManager(cfg, logger)
	assert.NoError(t, err)

	// 未连接时，producer 为 nil
	assert.Nil(t, manager.GetProducer())
}

func TestManager_GetConsumer_NotExists(t *testing.T) {
	logger := zap.NewNop()
	cfg := Config{
		Brokers: []string{"localhost:9092"},
	}

	manager, err := NewManager(cfg, logger)
	assert.NoError(t, err)

	// 不存在的消费者
	assert.Nil(t, manager.GetConsumer("nonexistent"))
}

func TestManager_CreateConsumer_Closed(t *testing.T) {
	logger := zap.NewNop()
	cfg := Config{
		Brokers: []string{"localhost:9092"},
	}

	manager, err := NewManager(cfg, logger)
	assert.NoError(t, err)

	// 关闭管理器
	manager.closed = true

	// 创建消费者应该失败
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

	// 模拟已存在的消费者
	manager.consumers["test"] = &ConsumerGroup{}

	// 创建同名消费者应该失败
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

	// 无效的消费者配置
	_, err = manager.CreateConsumer("test", ConsumerConfig{
		GroupID: "", // 空 group id
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

	// 第一次关闭
	err = manager.Close()
	assert.NoError(t, err)

	// 第二次关闭（幂等）
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

	// 模拟已有异步生产者
	manager.asyncProducer = &AsyncProducer{
		logger:    logger,
		successCh: make(chan *ProducerResult, 10),
		errorCh:   make(chan error, 10),
	}

	// 再次获取应该返回已有的
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

	// 使用已取消的 context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = manager.Ping(ctx)
	// 应该立即返回 context 错误或连接错误
	assert.Error(t, err)
}

func TestManager_ListTopics(t *testing.T) {
	logger := zap.NewNop()
	cfg := Config{
		Brokers: []string{"localhost:9092"},
	}

	manager, err := NewManager(cfg, logger)
	assert.NoError(t, err)

	// ListTopics 会尝试连接
	// 如果有真实 Kafka，会返回 topics 列表
	topics, err := manager.ListTopics(context.Background())
	// 不管成功还是失败，只测试方法可调用
	if err == nil {
		assert.NotNil(t, topics)
	}
}

// 测试 Ping 超时场景
func TestManager_Ping_ContextTimeout(t *testing.T) {
	logger := zap.NewNop()
	cfg := Config{
		Brokers: []string{"localhost:9092"},
	}

	manager, err := NewManager(cfg, logger)
	assert.NoError(t, err)

	// 创建一个已超时的 context
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(1 * time.Millisecond) // 确保超时

	err = manager.Ping(ctx)
	assert.Error(t, err)
}

// 测试真实连接场景（需要 Kafka 运行）
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
	// 如果 Kafka 运行，应该成功
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
	
	// 如果 Kafka 运行，应该成功
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

	// 首次获取异步生产者
	ap, err := manager.GetAsyncProducer()
	// 如果 Kafka 运行，应该成功
	if err == nil {
		assert.NotNil(t, ap)
		
		// 再次获取应该返回同一个
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

	// 连接
	err = manager.Connect(context.Background())
	assert.NoError(t, err)

	// 验证 producer
	producer := manager.GetProducer()
	assert.NotNil(t, producer)

	// Ping
	err = manager.Ping(context.Background())
	assert.NoError(t, err)

	// ListTopics
	topics, err := manager.ListTopics(context.Background())
	assert.NoError(t, err)
	t.Logf("Topics: %v", topics)

	// 发送消息
	result, err := producer.Send(context.Background(), &Message{
		Topic: "test-topic",
		Key:   []byte("full-flow-key"),
		Value: []byte("full-flow-value"),
	})
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// 创建消费者
	consumer, err := manager.CreateConsumer("flow-consumer", ConsumerConfig{
		GroupID:       "flow-group-" + time.Now().Format("150405"),
		Topics:        []string{"test-topic"},
		OffsetInitial: -2,
	})
	assert.NoError(t, err)
	assert.NotNil(t, consumer)

	// 验证 GetConfig
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

	// 创建消费者
	consumer, err := manager.CreateConsumer("close-test-consumer", ConsumerConfig{
		GroupID: "close-test-group",
		Topics:  []string{"test-topic"},
	})
	assert.NoError(t, err)

	// 启动消费者
	ctx, cancel := context.WithCancel(context.Background())
	err = consumer.Start(ctx, func(ctx context.Context, msg *ConsumedMessage) error {
		return nil
	})
	assert.NoError(t, err)

	// 获取异步生产者
	_, err = manager.GetAsyncProducer()
	assert.NoError(t, err)

	// 取消上下文
	cancel()

	// 等待一下让消费者停止
	time.Sleep(100 * time.Millisecond)

	// 关闭 - 应该关闭所有资源
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

	// 先连接
	err = manager.Connect(context.Background())
	assert.NoError(t, err)

	// Ping 应该成功
	err = manager.Ping(context.Background())
	assert.NoError(t, err)

	// 使用带超时的 context
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

	// 创建多个消费者
	for i := 0; i < 3; i++ {
		_, err = manager.CreateConsumer(fmt.Sprintf("consumer-%d", i), ConsumerConfig{
			GroupID: fmt.Sprintf("group-%d", i),
			Topics:  []string{"test-topic"},
		})
		assert.NoError(t, err)
	}

	// 关闭
	err = manager.Close()
	assert.NoError(t, err)

	// 再次关闭应该是幂等的
	err = manager.Close()
	assert.NoError(t, err)
}

