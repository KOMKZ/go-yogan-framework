package governance

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultHealthChecker_Check(t *testing.T) {
	checker := NewDefaultHealthChecker()

	err := checker.Check(context.Background())
	assert.NoError(t, err)
}

func TestDefaultHealthChecker_GetStatus(t *testing.T) {
	checker := NewDefaultHealthChecker()

	status := checker.GetStatus()
	assert.True(t, status.Healthy)
	assert.Equal(t, "OK", status.Message)
}
