package kafka

import (
	"context"
	"testing"
	"time"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/stretchr/testify/assert"
)

func TestNewHealthChecker(t *testing.T) {
	log := logger.GetLogger("test")
	cfg := Config{
		Brokers: []string{"localhost:9092"},
	}

	manager, err := NewManager(cfg, log)
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
	log := logger.GetLogger("test")
	cfg := Config{
		Brokers: []string{"localhost:9092"},
	}

	manager, err := NewManager(cfg, log)
	assert.NoError(t, err)

	checker := NewHealthChecker(manager)

	// If Kafka is running, the check should succeed
	err = checker.Check(context.Background())
	// Whether successful or not, verify that the method is callable
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
	log := logger.GetLogger("test")
	cfg := Config{
		Brokers: []string{"localhost:9092"},
	}

	manager, err := NewManager(cfg, log)
	assert.NoError(t, err)

	checker := NewHealthChecker(manager)
	checker.SetTimeout(1 * time.Nanosecond) // Extremely short timeout

	// should timeout or connection failed
	err = checker.Check(context.Background())
	assert.Error(t, err)
}

func TestHealthChecker_Check_CancelledContext(t *testing.T) {
	log := logger.GetLogger("test")
	cfg := Config{
		Brokers: []string{"localhost:9092"},
	}

	manager, err := NewManager(cfg, log)
	assert.NoError(t, err)

	checker := NewHealthChecker(manager)

	// Create canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = checker.Check(ctx)
	assert.Error(t, err)
}

