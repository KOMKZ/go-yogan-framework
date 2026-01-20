package httpx

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDefaultErrorLoggingConfig test default configuration
func TestDefaultErrorLoggingConfig(t *testing.T) {
	cfg := DefaultErrorLoggingConfig()

	assert.False(t, cfg.Enable)
	assert.Empty(t, cfg.IgnoreHTTPStatus)
	assert.True(t, cfg.FullErrorChain)
	assert.Equal(t, "error", cfg.LogLevel)
}

// TestErrorLoggingConfig_Fields Test configuration fields
func TestErrorLoggingConfig_Fields(t *testing.T) {
	cfg := ErrorLoggingConfig{
		Enable:           true,
		IgnoreHTTPStatus: []int{400, 404, 500},
		FullErrorChain:   false,
		LogLevel:         "warn",
	}

	assert.True(t, cfg.Enable)
	assert.Len(t, cfg.IgnoreHTTPStatus, 3)
	assert.Contains(t, cfg.IgnoreHTTPStatus, 400)
	assert.Contains(t, cfg.IgnoreHTTPStatus, 404)
	assert.Contains(t, cfg.IgnoreHTTPStatus, 500)
	assert.False(t, cfg.FullErrorChain)
	assert.Equal(t, "warn", cfg.LogLevel)
}
