// Provides unified health check capabilities
package health

import (
	"time"

	"github.com/KOMKZ/go-yogan-framework/component"
)

// Status health status enumeration
type Status string

const (
	// StatusHealthy Healthy
	StatusHealthy Status = "healthy"
	// StatusDegraded Degraded (partial functionality unavailable)
	StatusDegraded Status = "degraded"
	// StatusUnhealthy Unhealthy
	StatusUnhealthy Status = "unhealthy"
)

// Checker is an alias for component.HealthChecker for convenient use
type Checker = component.HealthChecker

// CheckResult individual check item result
type CheckResult struct {
	Name      string        `json:"name"`               // Check item name
	Status    Status        `json:"status"`             // health status
	Message   string        `json:"message,omitempty"`  // status message
	Error     string        `json:"error,omitempty"`    // Error message
	Timestamp time.Time     `json:"timestamp"`          // Check time
	Duration  time.Duration `json:"duration,omitempty"` // Check for time consumption
}

// Health check response
type Response struct {
	Status    Status                 `json:"status"`             // overall health status
	Timestamp time.Time              `json:"timestamp"`          // Check time
	Duration  time.Duration          `json:"duration"`           // Total check duration
	Checks    map[string]CheckResult `json:"checks"`             // Results of each inspection item
	Metadata  map[string]interface{} `json:"metadata,omitempty"` // metadata
}

// Check if the system is healthy overall
func (r *Response) IsHealthy() bool {
	return r.Status == StatusHealthy
}

// determines if degraded
func (r *Response) IsDegraded() bool {
	return r.Status == StatusDegraded
}
