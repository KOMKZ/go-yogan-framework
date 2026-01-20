package breaker

import (
	"sync"
	"time"
)

// stateManager state manager
type stateManager struct {
	state           State
	lastStateChange time.Time
	failureCount    int
	successCount    int
	halfOpenAttempts int
	mu              sync.RWMutex
}

// create state manager
func newStateManager() *stateManager {
	return &stateManager{
		state:           StateClosed,
		lastStateChange: time.Now(),
	}
}

// GetState Get current state (thread-safe)
func (sm *stateManager) GetState() State {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.state
}

// CanAttempt checks if attempting is allowed (based on current state and configuration)
func (sm *stateManager) CanAttempt(config ResourceConfig) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	switch sm.state {
	case StateClosed:
		// closed state, allow all requests
		return true
		
	case StateOpen:
		// Open state, check for timeout (can switch to half-open)
		if time.Since(sm.lastStateChange) >= config.Timeout {
			sm.transitionTo(StateHalfOpen, "timeout expired")
			sm.halfOpenAttempts = 0
			return true
		}
		// request has not timed out, reject request
		return false
		
	case StateHalfOpen:
		// half-open state, limit request count
		if sm.halfOpenAttempts < config.HalfOpenRequests {
			sm.halfOpenAttempts++
			return true
		}
		return false
		
	default:
		return false
	}
}

// RecordSuccess Recording successful
func (sm *stateManager) RecordSuccess(config ResourceConfig) (stateChanged bool, fromState, toState State) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	switch sm.state {
	case StateClosed:
		// close state, reset failure count
		sm.failureCount = 0
		
	case StateHalfOpen:
		// Half-open state, increase success count
		sm.successCount++
		if sm.successCount >= config.HalfOpenRequests {
			// Reach threshold, revert to closed state
			fromState = sm.state
			sm.transitionTo(StateClosed, "success threshold reached")
			sm.successCount = 0
			sm.failureCount = 0
			toState = sm.state
			return true, fromState, toState
		}
	}
	
	return false, sm.state, sm.state
}

// RecordFailure record failure
func (sm *stateManager) RecordFailure() (stateChanged bool, fromState, toState State) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	switch sm.state {
	case StateClosed:
		// close state, increment failure count
		sm.failureCount++
		
	case StateHalfOpen:
		// Half-open state failure, switch directly to open state
		fromState = sm.state
		sm.transitionTo(StateOpen, "failed in half-open state")
		sm.successCount = 0
		sm.failureCount = 0
		toState = sm.state
		return true, fromState, toState
	}
	
	return false, sm.state, sm.state
}

// ShouldOpen determines whether circuit breaking should be applied (requires external strategy judgment results)
func (sm *stateManager) ShouldOpen(shouldOpen bool) (stateChanged bool, fromState, toState State) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	if sm.state == StateClosed && shouldOpen {
		fromState = sm.state
		sm.transitionTo(StateOpen, "threshold exceeded")
		toState = sm.state
		return true, fromState, toState
	}
	
	return false, sm.state, sm.state
}

// Reset reset status
func (sm *stateManager) Reset() (stateChanged bool, fromState, toState State) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	if sm.state != StateClosed {
		fromState = sm.state
		sm.transitionTo(StateClosed, "manual reset")
		sm.failureCount = 0
		sm.successCount = 0
		sm.halfOpenAttempts = 0
		toState = sm.state
		return true, fromState, toState
	}
	
	return false, sm.state, sm.state
}

// transitionTo Switch state (internal method, lock required)
func (sm *stateManager) transitionTo(newState State, reason string) {
	sm.state = newState
	sm.lastStateChange = time.Now()
}

// GetFailureCount gets the failure count
func (sm *stateManager) GetFailureCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.failureCount
}

// GetSuccessCount Get successful count
func (sm *stateManager) GetSuccessCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.successCount
}

// Get last state change time
func (sm *stateManager) GetLastStateChange() time.Time {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.lastStateChange
}

