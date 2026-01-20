// Package breaker provides circuit breaker functionality
// 
// Design concept:
// - Standalone package, depends only on the logger component of yogan
// - Event-driven, the application layer can subscribe to all events
// - Metrics exposed, application layer can access and subscribe to real-time data
// - Optional enablement, does not take effect if not configured
package breaker

import (
	"context"
	"time"
)

// Circuit breaker core interface
type Breaker interface {
	// Execute protected call
	Execute(ctx context.Context, req *Request) (*Response, error)
	
	// GetState Retrieve the current state of the resource
	GetState(resource string) State
	
	// GetMetrics Obtain metric snapshot (accessible at the application layer)
	GetMetrics(resource string) *MetricsSnapshot
	
	// GetEventBus Obtain the event bus (for subscribing to events)
	GetEventBus() EventBus
	
	// GetMetricsCollector obtain metric collector (for subscribing to real-time data)
	GetMetricsCollector(resource string) MetricsCollector
	
	// Reset Manually reset the circuit breaker state
	Reset(resource string)
	
	// Close circuit breaker (clean up resources)
	Close() error
	
	// Check if the circuit breaker is enabled (returns false if not configured)
	IsEnabled() bool
}

// Request context
type Request struct {
	// Resource identifier (service name, method name, etc.)
	Resource string
	
	// Execute actual function call
	Execute func(ctx context.Context) (interface{}, error)
	
	// Fallback degradation logic (optional)
	Fallback func(ctx context.Context, err error) (interface{}, error)
	
	// Timeout (optional, 0 indicates using the configured timeout)
	Timeout time.Duration
}

// Response result
type Response struct {
	// Value return value
	Value interface{}
	
	// IsFromFallback whether from fallback
	FromFallback bool
	
	// Duration call time spent
	Duration time.Duration
	
	// Error (if any)
	Error error
}

// State circuit breaker status
type State int

const (
	// StateClosed Closed (normal)
	StateClosed State = iota
	
	// StateOpen Open (circuit breaker)
	StateOpen
	
	// StateHalfOpen Half open (recovery probe)
	StateHalfOpen
)

// Return status name
func (s State) String() string {
	switch s {
	case StateClosed:
		return "Closed"
	case StateOpen:
		return "Open"
	case StateHalfOpen:
		return "HalfOpen"
	default:
		return "Unknown"
	}
}

// IsOpen whether it is in circuit breaker state
func (s State) IsOpen() bool {
	return s == StateOpen
}

// IsClosed whether in normal state
func (s State) IsClosed() bool {
	return s == StateClosed
}

// IsHalfOpen whether it is in half-open state
func (s State) IsHalfOpen() bool {
	return s == StateHalfOpen
}

