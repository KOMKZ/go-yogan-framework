package database

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewHealthChecker tests health checker creation
func TestNewHealthChecker(t *testing.T) {
	configs := map[string]Config{
		"master": {
			Driver: "mysql",
			DSN:    testDSN,
		},
	}

	manager, err := NewManager(configs, nil, getTestLogger())
	if err != nil {
		t.Skipf("Skipping test: MySQL not available: %v", err)
	}
	require.NoError(t, err)
	defer manager.Close()

	checker := NewHealthChecker(manager)
	assert.NotNil(t, checker)
}

// TestHealthChecker_Name tests health checker name
func TestHealthChecker_Name(t *testing.T) {
	configs := map[string]Config{
		"master": {
			Driver: "mysql",
			DSN:    testDSN,
		},
	}

	manager, err := NewManager(configs, nil, getTestLogger())
	if err != nil {
		t.Skipf("Skipping test: MySQL not available: %v", err)
	}
	require.NoError(t, err)
	defer manager.Close()

	checker := NewHealthChecker(manager)
	name := checker.Name()
	assert.Equal(t, "database", name)
}

// TestHealthChecker_Check_Success tests successful health check
func TestHealthChecker_Check_Success(t *testing.T) {
	configs := map[string]Config{
		"master": {
			Driver: "mysql",
			DSN:    testDSN,
		},
	}

	manager, err := NewManager(configs, nil, getTestLogger())
	if err != nil {
		t.Skipf("Skipping test: MySQL not available: %v", err)
	}
	require.NoError(t, err)
	defer manager.Close()

	checker := NewHealthChecker(manager)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = checker.Check(ctx)
	assert.NoError(t, err)
}

// TestHealthChecker_Check_NilManager tests health check with nil manager
func TestHealthChecker_Check_NilManager(t *testing.T) {
	checker := NewHealthChecker(nil)

	ctx := context.Background()
	err := checker.Check(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database manager not initialized")
}
