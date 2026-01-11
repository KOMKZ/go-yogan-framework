package kafka

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestNewHealthChecker(t *testing.T) {
	logger := zap.NewNop()
	cfg := Config{
		Brokers: []string{"localhost:9092"},
	}

	manager, err := NewManager(cfg, logger)
	assert.NoError(t, err)

	checker := NewHealthChecker(manager)
	assert.NotNil(t, checker)
	assert.Equal(t, 5*time.Second, checker.timeout)
}

func TestHealthChecker_Name(t *testing.T) {
	checker := NewHealthChecker(nil)
	assert.Equal(t, "kafka", checker.Name())
}

func TestHealthChecker_Check_NilManager(t *testing.T) {
	checker := &HealthChecker{
		manager: nil,
		timeout: 5 * time.Second,
	}

	err := checker.Check(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "kafka manager is nil")
}

func TestHealthChecker_Check_WithManager(t *testing.T) {
	logger := zap.NewNop()
	cfg := Config{
		Brokers: []string{"localhost:9092"},
	}

	manager, err := NewManager(cfg, logger)
	assert.NoError(t, err)

	checker := NewHealthChecker(manager)

	// 如果 Kafka 运行中，检查应该成功
	err = checker.Check(context.Background())
	// 不论成功失败，都验证方法可调用
	if err != nil {
		t.Log("Health check failed (Kafka may not be available):", err)
	}
}

func TestHealthChecker_SetTimeout(t *testing.T) {
	checker := &HealthChecker{
		timeout: 5 * time.Second,
	}

	checker.SetTimeout(10 * time.Second)
	assert.Equal(t, 10*time.Second, checker.timeout)
}

func TestHealthChecker_Check_ContextTimeout(t *testing.T) {
	logger := zap.NewNop()
	cfg := Config{
		Brokers: []string{"localhost:9092"},
	}

	manager, err := NewManager(cfg, logger)
	assert.NoError(t, err)

	checker := NewHealthChecker(manager)
	checker.SetTimeout(1 * time.Nanosecond) // 极短超时

	// 应该超时或连接失败
	err = checker.Check(context.Background())
	assert.Error(t, err)
}

func TestHealthChecker_Check_CancelledContext(t *testing.T) {
	logger := zap.NewNop()
	cfg := Config{
		Brokers: []string{"localhost:9092"},
	}

	manager, err := NewManager(cfg, logger)
	assert.NoError(t, err)

	checker := NewHealthChecker(manager)

	// 创建已取消的 context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = checker.Check(ctx)
	assert.Error(t, err)
}

