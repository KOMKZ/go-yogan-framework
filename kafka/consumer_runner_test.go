package kafka

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConsumerRunnerConfig_ApplyDefaults(t *testing.T) {
	tests := []struct {
		name        string
		input       ConsumerRunnerConfig
		handlerName string
		wantGroupID string
		wantWorkers int
		wantOffset  int64
	}{
		{
			name:        "empty config",
			input:       ConsumerRunnerConfig{},
			handlerName: "test",
			wantGroupID: "test-group",
			wantWorkers: 1,
			wantOffset:  -1,
		},
		{
			name: "partial config",
			input: ConsumerRunnerConfig{
				Workers: 3,
			},
			handlerName: "demo",
			wantGroupID: "demo-group",
			wantWorkers: 3,
			wantOffset:  -1,
		},
		{
			name: "full config",
			input: ConsumerRunnerConfig{
				GroupID:       "custom-group",
				Workers:       5,
				OffsetInitial: -2,
			},
			handlerName: "test",
			wantGroupID: "custom-group",
			wantWorkers: 5,
			wantOffset:  -2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.input
			cfg.applyDefaults(tt.handlerName)

			assert.Equal(t, tt.wantGroupID, cfg.GroupID)
			assert.Equal(t, tt.wantWorkers, cfg.Workers)
			assert.Equal(t, tt.wantOffset, cfg.OffsetInitial)
			assert.True(t, cfg.AutoCommitInterval > 0)
			assert.True(t, cfg.MaxProcessingTime > 0)
			assert.True(t, cfg.SessionTimeout > 0)
			assert.True(t, cfg.HeartbeatInterval > 0)
		})
	}
}

func TestNewConsumerRunner(t *testing.T) {
	// 创建 mock manager (需要真实 logger)
	// 这里我们只测试配置应用
	handler := NewConsumerHandlerFunc("test-runner", []string{"topic1"}, nil)
	cfg := ConsumerRunnerConfig{
		Workers: 2,
	}

	// 由于需要真实 Manager，这里只测试配置部分
	cfg.applyDefaults(handler.Name())
	assert.Equal(t, "test-runner-group", cfg.GroupID)
	assert.Equal(t, 2, cfg.Workers)
}

func TestConsumerRunner_IsRunning_Initial(t *testing.T) {
	// 测试初始状态
	runner := &ConsumerRunner{}
	assert.False(t, runner.IsRunning())
}

func TestConsumerRunner_GetConfig(t *testing.T) {
	cfg := ConsumerRunnerConfig{
		GroupID:            "test-group",
		Workers:            3,
		OffsetInitial:      -2,
		AutoCommit:         true,
		AutoCommitInterval: 2 * time.Second,
	}

	runner := &ConsumerRunner{config: cfg}
	got := runner.GetConfig()

	assert.Equal(t, "test-group", got.GroupID)
	assert.Equal(t, 3, got.Workers)
	assert.Equal(t, int64(-2), got.OffsetInitial)
	assert.True(t, got.AutoCommit)
	assert.Equal(t, 2*time.Second, got.AutoCommitInterval)
}

// 测试 Handler 包装
func TestConsumerHandlerWithContext(t *testing.T) {
	var receivedCtx context.Context
	handler := NewConsumerHandlerFunc("test", []string{"topic"}, func(ctx context.Context, msg *ConsumedMessage) error {
		receivedCtx = ctx
		return nil
	})

	ctx := context.WithValue(context.Background(), "key", "value")
	err := handler.Handle(ctx, &ConsumedMessage{})

	assert.NoError(t, err)
	assert.Equal(t, "value", receivedCtx.Value("key"))
}
