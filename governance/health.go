package governance

import (
	"context"
)

// HealthChecker health check interface
type HealthChecker interface {
	// Check execution health check
	// Return nil indicates healthy, return error indicates unhealthy
	Check(ctx context.Context) error

	// GetStatus Retrieve health status
	GetStatus() HealthStatus
}

// HealthStatus health status
type HealthStatus struct {
	Healthy bool              `json:"healthy"` // Is healthy
	Message string            `json:"message"` // status message
	Details map[string]string `json:"details"` // Detailed information
}

// DefaultHealthChecker default health checker (always returns healthy)
type DefaultHealthChecker struct{}

// Create default health checker
func NewDefaultHealthChecker() *DefaultHealthChecker {
	return &DefaultHealthChecker{}
}

// Check execution health (default implementation: always healthy)
func (h *DefaultHealthChecker) Check(ctx context.Context) error {
	return nil
}

// GetStatus Retrieve health status
func (h *DefaultHealthChecker) GetStatus() HealthStatus {
	return HealthStatus{
		Healthy: true,
		Message: "OK",
	}
}

