package governance

import (
	"fmt"
	"sync"
	"time"
)

// CircuitState circuit breaker status
type CircuitState int

const (
	StateClosed   CircuitState = iota // Close (normal)
	StateHalfOpen                     // half-open (attempting recovery)
	StateOpen                         // Enable (circuit breaker)
)

// Return status name
func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "Closed"
	case StateHalfOpen:
		return "HalfOpen"
	case StateOpen:
		return "Open"
	default:
		return "Unknown"
	}
}

// CircuitBreakerConfig circuit breaker configuration
type CircuitBreakerConfig struct {
	FailureThreshold int           // Failure threshold (circuit breaker trigger)
	SuccessThreshold int           // Success threshold value (for recovery)
	Timeout          time.Duration // circuit breaker duration
	HalfOpenRequests int           // Number of requests allowed when half-open
}

// Default Circuit Breaker Configuration
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 2,
		Timeout:          30 * time.Second,
		HalfOpenRequests: 3,
	}
}

// CircuitBreaker circuit breaker interface
type CircuitBreaker interface {
	// Call with circuit breaker protection
	Call(serviceName string, fn func() error) error

	// RecordSuccess Recording successful
	RecordSuccess(serviceName string)

	// RecordFailure log failure
	RecordFailure(serviceName string)

	// GetState 获取状态英文为 Get State 或者直接 GetStatus
	GetState(serviceName string) CircuitState

	// Reset circuit breaker
	Reset(serviceName string)
}

// circuitBreakerState circuit breaker status of a single service
type circuitBreakerState struct {
	state            CircuitState
	failureCount     int
	successCount     int
	lastStateChange  time.Time
	halfOpenAttempts int
}

// SimpleCircuitBreaker simple circuit breaker implementation
type SimpleCircuitBreaker struct {
	config CircuitBreakerConfig
	states map[string]*circuitBreakerState
	mu     sync.RWMutex
}

// Create simple circuit breaker
func NewSimpleCircuitBreaker(config CircuitBreakerConfig) *SimpleCircuitBreaker {
	return &SimpleCircuitBreaker{
		config: config,
		states: make(map[string]*circuitBreakerState),
	}
}

// Call with circuit breaker protection
func (cb *SimpleCircuitBreaker) Call(serviceName string, fn func() error) error {
	// Check circuit breaker status
	if !cb.allowRequest(serviceName) {
		return fmt.Errorf("circuit breaker is open for service: %s", serviceName)
	}

	// Execute call
	err := fn()

	// Record the result
	if err != nil {
		cb.RecordFailure(serviceName)
	} else {
		cb.RecordSuccess(serviceName)
	}

	return err
}

// allowRequest checks if the request is allowed
func (cb *SimpleCircuitBreaker) allowRequest(serviceName string) bool {
	cb.mu.RLock()
	state := cb.getOrCreateState(serviceName)
	cb.mu.RUnlock()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch state.state {
	case StateClosed:
		// closed state, allow all requests
		return true

	case StateOpen:
		// Open state, check for timeout
		if time.Since(state.lastStateChange) >= cb.config.Timeout {
			// Timeout, switch to half-open state
			state.state = StateHalfOpen
			state.halfOpenAttempts = 0
			state.lastStateChange = time.Now()
			return true
		}
		// request not timed out, reject request
		return false

	case StateHalfOpen:
		// Half-open state, limit request count
		if state.halfOpenAttempts < cb.config.HalfOpenRequests {
			state.halfOpenAttempts++
			return true
		}
		return false

	default:
		return true
	}
}

// RecordSuccess Recording successful
func (cb *SimpleCircuitBreaker) RecordSuccess(serviceName string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	state := cb.getOrCreateState(serviceName)

	switch state.state {
	case StateClosed:
		// close state, reset failure count
		state.failureCount = 0

	case StateHalfOpen:
		// half-open state, increment success count
		state.successCount++
		if state.successCount >= cb.config.SuccessThreshold {
			// Reach threshold, revert to closed state
			state.state = StateClosed
			state.successCount = 0
			state.failureCount = 0
			state.lastStateChange = time.Now()
		}
	}
}

// RecordFailure record failure
func (cb *SimpleCircuitBreaker) RecordFailure(serviceName string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	state := cb.getOrCreateState(serviceName)

	switch state.state {
	case StateClosed:
		// close state, increase failure count
		state.failureCount++
		if state.failureCount >= cb.config.FailureThreshold {
			// Reach threshold, switch to open state
			state.state = StateOpen
			state.lastStateChange = time.Now()
		}

	case StateHalfOpen:
		// semi-open state failed, switch directly to open state
		state.state = StateOpen
		state.successCount = 0
		state.failureCount = 0
		state.lastStateChange = time.Now()
	}
}

// GetState Get status
func (cb *SimpleCircuitBreaker) GetState(serviceName string) CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	state := cb.getOrCreateState(serviceName)
	return state.state
}

// Reset circuit breaker
func (cb *SimpleCircuitBreaker) Reset(serviceName string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	state := cb.getOrCreateState(serviceName)
	state.state = StateClosed
	state.failureCount = 0
	state.successCount = 0
	state.lastStateChange = time.Now()
}

// getOrCreateState Get or create state (lock required)
func (cb *SimpleCircuitBreaker) getOrCreateState(serviceName string) *circuitBreakerState {
	state, exists := cb.states[serviceName]
	if !exists {
		state = &circuitBreakerState{
			state:           StateClosed,
			lastStateChange: time.Now(),
		}
		cb.states[serviceName] = state
	}
	return state
}

