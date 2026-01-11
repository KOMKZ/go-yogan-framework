package governance

import (
	"fmt"
	"sync"
	"time"
)

// CircuitState 熔断器状态
type CircuitState int

const (
	StateClosed   CircuitState = iota // 关闭（正常）
	StateHalfOpen                     // 半开（尝试恢复）
	StateOpen                         // 打开（熔断）
)

// String 返回状态名称
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

// CircuitBreakerConfig 熔断器配置
type CircuitBreakerConfig struct {
	FailureThreshold int           // 失败次数阈值（触发熔断）
	SuccessThreshold int           // 成功次数阈值（恢复）
	Timeout          time.Duration // 熔断时长
	HalfOpenRequests int           // 半开时允许的请求数
}

// DefaultCircuitBreakerConfig 默认熔断器配置
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 2,
		Timeout:          30 * time.Second,
		HalfOpenRequests: 3,
	}
}

// CircuitBreaker 熔断器接口
type CircuitBreaker interface {
	// Call 执行调用（带熔断保护）
	Call(serviceName string, fn func() error) error

	// RecordSuccess 记录成功
	RecordSuccess(serviceName string)

	// RecordFailure 记录失败
	RecordFailure(serviceName string)

	// GetState 获取状态
	GetState(serviceName string) CircuitState

	// Reset 重置熔断器
	Reset(serviceName string)
}

// circuitBreakerState 单个服务的熔断器状态
type circuitBreakerState struct {
	state            CircuitState
	failureCount     int
	successCount     int
	lastStateChange  time.Time
	halfOpenAttempts int
}

// SimpleCircuitBreaker 简单熔断器实现
type SimpleCircuitBreaker struct {
	config CircuitBreakerConfig
	states map[string]*circuitBreakerState
	mu     sync.RWMutex
}

// NewSimpleCircuitBreaker 创建简单熔断器
func NewSimpleCircuitBreaker(config CircuitBreakerConfig) *SimpleCircuitBreaker {
	return &SimpleCircuitBreaker{
		config: config,
		states: make(map[string]*circuitBreakerState),
	}
}

// Call 执行调用（带熔断保护）
func (cb *SimpleCircuitBreaker) Call(serviceName string, fn func() error) error {
	// 检查熔断状态
	if !cb.allowRequest(serviceName) {
		return fmt.Errorf("circuit breaker is open for service: %s", serviceName)
	}

	// 执行调用
	err := fn()

	// 记录结果
	if err != nil {
		cb.RecordFailure(serviceName)
	} else {
		cb.RecordSuccess(serviceName)
	}

	return err
}

// allowRequest 检查是否允许请求
func (cb *SimpleCircuitBreaker) allowRequest(serviceName string) bool {
	cb.mu.RLock()
	state := cb.getOrCreateState(serviceName)
	cb.mu.RUnlock()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch state.state {
	case StateClosed:
		// 关闭状态，允许所有请求
		return true

	case StateOpen:
		// 打开状态，检查是否超时
		if time.Since(state.lastStateChange) >= cb.config.Timeout {
			// 超时，切换到半开状态
			state.state = StateHalfOpen
			state.halfOpenAttempts = 0
			state.lastStateChange = time.Now()
			return true
		}
		// 未超时，拒绝请求
		return false

	case StateHalfOpen:
		// 半开状态，限制请求数
		if state.halfOpenAttempts < cb.config.HalfOpenRequests {
			state.halfOpenAttempts++
			return true
		}
		return false

	default:
		return true
	}
}

// RecordSuccess 记录成功
func (cb *SimpleCircuitBreaker) RecordSuccess(serviceName string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	state := cb.getOrCreateState(serviceName)

	switch state.state {
	case StateClosed:
		// 关闭状态，重置失败计数
		state.failureCount = 0

	case StateHalfOpen:
		// 半开状态，增加成功计数
		state.successCount++
		if state.successCount >= cb.config.SuccessThreshold {
			// 达到阈值，恢复到关闭状态
			state.state = StateClosed
			state.successCount = 0
			state.failureCount = 0
			state.lastStateChange = time.Now()
		}
	}
}

// RecordFailure 记录失败
func (cb *SimpleCircuitBreaker) RecordFailure(serviceName string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	state := cb.getOrCreateState(serviceName)

	switch state.state {
	case StateClosed:
		// 关闭状态，增加失败计数
		state.failureCount++
		if state.failureCount >= cb.config.FailureThreshold {
			// 达到阈值，切换到打开状态
			state.state = StateOpen
			state.lastStateChange = time.Now()
		}

	case StateHalfOpen:
		// 半开状态失败，直接切换到打开状态
		state.state = StateOpen
		state.successCount = 0
		state.failureCount = 0
		state.lastStateChange = time.Now()
	}
}

// GetState 获取状态
func (cb *SimpleCircuitBreaker) GetState(serviceName string) CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	state := cb.getOrCreateState(serviceName)
	return state.state
}

// Reset 重置熔断器
func (cb *SimpleCircuitBreaker) Reset(serviceName string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	state := cb.getOrCreateState(serviceName)
	state.state = StateClosed
	state.failureCount = 0
	state.successCount = 0
	state.lastStateChange = time.Now()
}

// getOrCreateState 获取或创建状态（需要持有锁）
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

