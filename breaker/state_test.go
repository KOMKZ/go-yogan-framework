package breaker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestNewStateManager 测试创建状态管理器
func TestNewStateManager(t *testing.T) {
	sm := newStateManager()
	
	assert.NotNil(t, sm)
	assert.Equal(t, StateClosed, sm.GetState())
	assert.Equal(t, 0, sm.GetFailureCount())
	assert.Equal(t, 0, sm.GetSuccessCount())
}

// TestStateManager_GetState 测试获取状态
func TestStateManager_GetState(t *testing.T) {
	sm := newStateManager()
	
	assert.Equal(t, StateClosed, sm.GetState())
	
	sm.mu.Lock()
	sm.state = StateOpen
	sm.mu.Unlock()
	
	assert.Equal(t, StateOpen, sm.GetState())
}

// TestStateManager_CanAttempt 测试是否允许尝试
func TestStateManager_CanAttempt(t *testing.T) {
	config := DefaultResourceConfig()
	config.Timeout = 100 * time.Millisecond
	config.HalfOpenRequests = 3
	
	t.Run("Closed状态允许请求", func(t *testing.T) {
		sm := newStateManager()
		assert.True(t, sm.CanAttempt(config))
	})
	
	t.Run("Open状态未超时拒绝请求", func(t *testing.T) {
		sm := newStateManager()
		sm.mu.Lock()
		sm.state = StateOpen
		sm.lastStateChange = time.Now()
		sm.mu.Unlock()
		
		assert.False(t, sm.CanAttempt(config))
	})
	
	t.Run("Open状态超时后切换到HalfOpen", func(t *testing.T) {
		sm := newStateManager()
		sm.mu.Lock()
		sm.state = StateOpen
		sm.lastStateChange = time.Now().Add(-200 * time.Millisecond)
		sm.mu.Unlock()
		
		assert.True(t, sm.CanAttempt(config))
		assert.Equal(t, StateHalfOpen, sm.GetState())
	})
	
	t.Run("HalfOpen状态限制请求数", func(t *testing.T) {
		sm := newStateManager()
		sm.mu.Lock()
		sm.state = StateHalfOpen
		sm.halfOpenAttempts = 0
		sm.mu.Unlock()
		
		// 前3次请求应该被允许
		for i := 0; i < 3; i++ {
			assert.True(t, sm.CanAttempt(config), "第%d次请求", i+1)
		}
		
		// 第4次请求应该被拒绝
		assert.False(t, sm.CanAttempt(config))
	})
}

// TestStateManager_RecordSuccess 测试记录成功
func TestStateManager_RecordSuccess(t *testing.T) {
	config := DefaultResourceConfig()
	config.HalfOpenRequests = 2
	
	t.Run("Closed状态记录成功重置失败计数", func(t *testing.T) {
		sm := newStateManager()
		sm.mu.Lock()
		sm.failureCount = 5
		sm.mu.Unlock()
		
		changed, _, _ := sm.RecordSuccess(config)
		
		assert.False(t, changed)
		assert.Equal(t, 0, sm.GetFailureCount())
		assert.Equal(t, StateClosed, sm.GetState())
	})
	
	t.Run("HalfOpen状态记录成功未达阈值", func(t *testing.T) {
		sm := newStateManager()
		sm.mu.Lock()
		sm.state = StateHalfOpen
		sm.successCount = 0
		sm.mu.Unlock()
		
		changed, _, _ := sm.RecordSuccess(config)
		
		assert.False(t, changed)
		assert.Equal(t, 1, sm.GetSuccessCount())
		assert.Equal(t, StateHalfOpen, sm.GetState())
	})
	
	t.Run("HalfOpen状态记录成功达到阈值切换到Closed", func(t *testing.T) {
		sm := newStateManager()
		sm.mu.Lock()
		sm.state = StateHalfOpen
		sm.successCount = 1
		sm.failureCount = 3
		sm.mu.Unlock()
		
		changed, fromState, toState := sm.RecordSuccess(config)
		
		assert.True(t, changed)
		assert.Equal(t, StateHalfOpen, fromState)
		assert.Equal(t, StateClosed, toState)
		assert.Equal(t, StateClosed, sm.GetState())
		assert.Equal(t, 0, sm.GetSuccessCount())
		assert.Equal(t, 0, sm.GetFailureCount())
	})
}

// TestStateManager_RecordFailure 测试记录失败
func TestStateManager_RecordFailure(t *testing.T) {
	t.Run("Closed状态记录失败增加计数", func(t *testing.T) {
		sm := newStateManager()
		
		changed, _, _ := sm.RecordFailure()
		
		assert.False(t, changed)
		assert.Equal(t, 1, sm.GetFailureCount())
		assert.Equal(t, StateClosed, sm.GetState())
	})
	
	t.Run("HalfOpen状态记录失败切换到Open", func(t *testing.T) {
		sm := newStateManager()
		sm.mu.Lock()
		sm.state = StateHalfOpen
		sm.successCount = 1
		sm.failureCount = 2
		sm.mu.Unlock()
		
		changed, fromState, toState := sm.RecordFailure()
		
		assert.True(t, changed)
		assert.Equal(t, StateHalfOpen, fromState)
		assert.Equal(t, StateOpen, toState)
		assert.Equal(t, StateOpen, sm.GetState())
		assert.Equal(t, 0, sm.GetSuccessCount())
		assert.Equal(t, 0, sm.GetFailureCount())
	})
	
	t.Run("Open状态记录失败无变化", func(t *testing.T) {
		sm := newStateManager()
		sm.mu.Lock()
		sm.state = StateOpen
		sm.mu.Unlock()
		
		changed, _, _ := sm.RecordFailure()
		
		assert.False(t, changed)
		assert.Equal(t, StateOpen, sm.GetState())
	})
}

// TestStateManager_ShouldOpen 测试判断是否应该熔断
func TestStateManager_ShouldOpen(t *testing.T) {
	t.Run("Closed状态达到阈值切换到Open", func(t *testing.T) {
		sm := newStateManager()
		
		changed, fromState, toState := sm.ShouldOpen(true)
		
		assert.True(t, changed)
		assert.Equal(t, StateClosed, fromState)
		assert.Equal(t, StateOpen, toState)
		assert.Equal(t, StateOpen, sm.GetState())
	})
	
	t.Run("Closed状态未达阈值无变化", func(t *testing.T) {
		sm := newStateManager()
		
		changed, _, _ := sm.ShouldOpen(false)
		
		assert.False(t, changed)
		assert.Equal(t, StateClosed, sm.GetState())
	})
	
	t.Run("Open状态不再切换", func(t *testing.T) {
		sm := newStateManager()
		sm.mu.Lock()
		sm.state = StateOpen
		sm.mu.Unlock()
		
		changed, _, _ := sm.ShouldOpen(true)
		
		assert.False(t, changed)
		assert.Equal(t, StateOpen, sm.GetState())
	})
	
	t.Run("HalfOpen状态不切换", func(t *testing.T) {
		sm := newStateManager()
		sm.mu.Lock()
		sm.state = StateHalfOpen
		sm.mu.Unlock()
		
		changed, _, _ := sm.ShouldOpen(true)
		
		assert.False(t, changed)
		assert.Equal(t, StateHalfOpen, sm.GetState())
	})
}

// TestStateManager_Reset 测试重置状态
func TestStateManager_Reset(t *testing.T) {
	t.Run("从Open状态重置到Closed", func(t *testing.T) {
		sm := newStateManager()
		sm.mu.Lock()
		sm.state = StateOpen
		sm.failureCount = 10
		sm.successCount = 5
		sm.halfOpenAttempts = 2
		sm.mu.Unlock()
		
		changed, fromState, toState := sm.Reset()
		
		assert.True(t, changed)
		assert.Equal(t, StateOpen, fromState)
		assert.Equal(t, StateClosed, toState)
		assert.Equal(t, StateClosed, sm.GetState())
		assert.Equal(t, 0, sm.GetFailureCount())
		assert.Equal(t, 0, sm.GetSuccessCount())
	})
	
	t.Run("从HalfOpen状态重置到Closed", func(t *testing.T) {
		sm := newStateManager()
		sm.mu.Lock()
		sm.state = StateHalfOpen
		sm.mu.Unlock()
		
		changed, fromState, toState := sm.Reset()
		
		assert.True(t, changed)
		assert.Equal(t, StateHalfOpen, fromState)
		assert.Equal(t, StateClosed, toState)
	})
	
	t.Run("Closed状态重置无变化", func(t *testing.T) {
		sm := newStateManager()
		
		changed, _, _ := sm.Reset()
		
		assert.False(t, changed)
		assert.Equal(t, StateClosed, sm.GetState())
	})
}

// TestStateManager_Concurrent 测试并发安全
func TestStateManager_Concurrent(t *testing.T) {
	sm := newStateManager()
	config := DefaultResourceConfig()
	
	done := make(chan bool)
	
	// 并发读取状态
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = sm.GetState()
				_ = sm.GetFailureCount()
				_ = sm.GetSuccessCount()
			}
			done <- true
		}()
	}
	
	// 并发修改状态
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = sm.CanAttempt(config)
				sm.RecordSuccess(config)
				sm.RecordFailure()
			}
			done <- true
		}()
	}
	
	// 等待完成
	for i := 0; i < 20; i++ {
		<-done
	}
	
	// 验证状态是有效的
	state := sm.GetState()
	assert.True(t, state == StateClosed || state == StateOpen || state == StateHalfOpen)
}

// TestStateManager_GetLastStateChange 测试获取最后状态变化时间
func TestStateManager_GetLastStateChange(t *testing.T) {
	sm := newStateManager()
	
	before := time.Now()
	time.Sleep(10 * time.Millisecond)
	
	sm.mu.Lock()
	sm.transitionTo(StateOpen, "test")
	sm.mu.Unlock()
	
	time.Sleep(10 * time.Millisecond)
	after := time.Now()
	
	lastChange := sm.GetLastStateChange()
	assert.True(t, lastChange.After(before))
	assert.True(t, lastChange.Before(after))
}

