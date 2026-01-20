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
	// Create mock manager (real logger required)
	// Here we only test the configuration of the application
	handler := NewConsumerHandlerFunc("test-runner", []string{"topic1"}, nil)
	cfg := ConsumerRunnerConfig{
		Workers: 2,
	}

	// Since a real Manager is required, here only test the configuration part
	cfg.applyDefaults(handler.Name())
	assert.Equal(t, "test-runner-group", cfg.GroupID)
	assert.Equal(t, 2, cfg.Workers)
}

func TestConsumerRunner_IsRunning_Initial(t *testing.T) {
	// Test initial state
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

// Test Handler wrapping
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
