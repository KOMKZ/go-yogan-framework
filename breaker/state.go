package breaker

import (
	"sync"
	"time"
)

// stateManager 状态管理器
type stateManager struct {
	state           State
	lastStateChange time.Time
	failureCount    int
	successCount    int
	halfOpenAttempts int
	mu              sync.RWMutex
}

// newStateManager 创建状态管理器
func newStateManager() *stateManager {
	return &stateManager{
		state:           StateClosed,
		lastStateChange: time.Now(),
	}
}

// GetState 获取当前状态（线程安全）
func (sm *stateManager) GetState() State {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.state
}

// CanAttempt 判断是否允许尝试（根据当前状态和配置）
func (sm *stateManager) CanAttempt(config ResourceConfig) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	switch sm.state {
	case StateClosed:
		// 关闭状态，允许所有请求
		return true
		
	case StateOpen:
		// 打开状态，检查是否超时（可以切换到半开）
		if time.Since(sm.lastStateChange) >= config.Timeout {
			sm.transitionTo(StateHalfOpen, "timeout expired")
			sm.halfOpenAttempts = 0
			return true
		}
		// 未超时，拒绝请求
		return false
		
	case StateHalfOpen:
		// 半开状态，限制请求数
		if sm.halfOpenAttempts < config.HalfOpenRequests {
			sm.halfOpenAttempts++
			return true
		}
		return false
		
	default:
		return false
	}
}

// RecordSuccess 记录成功
func (sm *stateManager) RecordSuccess(config ResourceConfig) (stateChanged bool, fromState, toState State) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	switch sm.state {
	case StateClosed:
		// 关闭状态，重置失败计数
		sm.failureCount = 0
		
	case StateHalfOpen:
		// 半开状态，增加成功计数
		sm.successCount++
		if sm.successCount >= config.HalfOpenRequests {
			// 达到阈值，恢复到关闭状态
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

// RecordFailure 记录失败
func (sm *stateManager) RecordFailure() (stateChanged bool, fromState, toState State) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	switch sm.state {
	case StateClosed:
		// 关闭状态，增加失败计数
		sm.failureCount++
		
	case StateHalfOpen:
		// 半开状态失败，直接切换到打开状态
		fromState = sm.state
		sm.transitionTo(StateOpen, "failed in half-open state")
		sm.successCount = 0
		sm.failureCount = 0
		toState = sm.state
		return true, fromState, toState
	}
	
	return false, sm.state, sm.state
}

// ShouldOpen 判断是否应该熔断（需要外部传入策略判断结果）
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

// Reset 重置状态
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

// transitionTo 切换状态（内部方法，需要持有锁）
func (sm *stateManager) transitionTo(newState State, reason string) {
	sm.state = newState
	sm.lastStateChange = time.Now()
}

// GetFailureCount 获取失败次数
func (sm *stateManager) GetFailureCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.failureCount
}

// GetSuccessCount 获取成功次数
func (sm *stateManager) GetSuccessCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.successCount
}

// GetLastStateChange 获取最后状态变化时间
func (sm *stateManager) GetLastStateChange() time.Time {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.lastStateChange
}

