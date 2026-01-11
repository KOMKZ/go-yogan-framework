package breaker

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestNewManager 测试创建管理器
func TestNewManager(t *testing.T) {
	t.Run("创建启用的管理器", func(t *testing.T) {
		config := DefaultConfig()
		config.Enabled = true
		
		mgr, err := NewManager(config)
		assert.NoError(t, err)
		assert.NotNil(t, mgr)
		assert.NotNil(t, mgr.eventBus)
		defer mgr.Close()
	})
	
	t.Run("创建未启用的管理器", func(t *testing.T) {
		config := DefaultConfig()
		config.Enabled = false
		
		mgr, err := NewManager(config)
		assert.NoError(t, err)
		assert.NotNil(t, mgr)
	})
	
	t.Run("无效配置返回错误", func(t *testing.T) {
		config := DefaultConfig()
		config.Enabled = true
		config.Default.MinRequests = -1 // 无效配置
		
		mgr, err := NewManager(config)
		assert.Error(t, err)
		assert.Nil(t, mgr)
	})
}

// TestManager_Execute_Disabled 测试未启用时直接透传
func TestManager_Execute_Disabled(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = false
	
	mgr, _ := NewManager(config)
	
	called := false
	req := &Request{
		Resource: "test",
		Execute: func(ctx context.Context) (interface{}, error) {
			called = true
			return "result", nil
		},
	}
	
	result, err := mgr.Execute(context.Background(), req)
	
	assert.NoError(t, err)
	assert.Equal(t, "result", result)
	assert.True(t, called)
}

// TestManager_Execute_Success 测试成功执行
func TestManager_Execute_Success(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	
	mgr, _ := NewManager(config)
	defer mgr.Close()
	
	req := &Request{
		Resource: "test-service",
		Execute: func(ctx context.Context) (interface{}, error) {
			return "success", nil
		},
	}
	
	result, err := mgr.Execute(context.Background(), req)
	
	assert.NoError(t, err)
	assert.Equal(t, "success", result)
	
	// 验证熔断器被创建
	state := mgr.GetState("test-service")
	assert.Equal(t, StateClosed, state)
}

// TestManager_Execute_Failure 测试失败执行
func TestManager_Execute_Failure(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	
	mgr, _ := NewManager(config)
	defer mgr.Close()
	
	expectedErr := errors.New("test error")
	req := &Request{
		Resource: "test-service",
		Execute: func(ctx context.Context) (interface{}, error) {
			return nil, expectedErr
		},
	}
	
	result, err := mgr.Execute(context.Background(), req)
	
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Nil(t, result)
}

// TestManager_Execute_WithFallback 测试带降级的执行
func TestManager_Execute_WithFallback(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Default.ErrorRateThreshold = 0.5
	config.Default.MinRequests = 5
	
	mgr, _ := NewManager(config)
	defer mgr.Close()
	
	req := &Request{
		Resource: "test-service",
		Execute: func(ctx context.Context) (interface{}, error) {
			return nil, errors.New("service error")
		},
		Fallback: func(ctx context.Context, err error) (interface{}, error) {
			return "fallback result", nil
		},
	}
	
	// 第一次失败，应该执行降级
	result, err := mgr.Execute(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, "fallback result", result)
}

// TestCircuitBreaker_StateTransition 测试状态转换
func TestCircuitBreaker_StateTransition(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Default.ErrorRateThreshold = 0.5
	config.Default.MinRequests = 10
	config.Default.Timeout = 100 * time.Millisecond
	
	mgr, _ := NewManager(config)
	defer mgr.Close()
	
	// 初始状态应该是 Closed
	breaker := mgr.getOrCreateBreaker("test")
	assert.Equal(t, StateClosed, breaker.GetState())
	
	// 模拟10次请求，6次失败
	for i := 0; i < 4; i++ {
		req := &Request{
			Resource: "test",
			Execute: func(ctx context.Context) (interface{}, error) {
				return "ok", nil
			},
		}
		mgr.Execute(context.Background(), req)
	}
	
	for i := 0; i < 6; i++ {
		req := &Request{
			Resource: "test",
			Execute: func(ctx context.Context) (interface{}, error) {
				return nil, errors.New("error")
			},
		}
		mgr.Execute(context.Background(), req)
	}
	
	// 应该触发熔断，状态变为 Open
	assert.Equal(t, StateOpen, breaker.GetState())
	
	// 等待超时后，状态应该变为 HalfOpen
	time.Sleep(150 * time.Millisecond)
	
	req := &Request{
		Resource: "test",
		Execute: func(ctx context.Context) (interface{}, error) {
			return "ok", nil
		},
	}
	mgr.Execute(context.Background(), req)
	
	assert.Equal(t, StateHalfOpen, breaker.GetState())
}

// TestCircuitBreaker_EventPublish 测试事件发布
func TestCircuitBreaker_EventPublish(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	
	mgr, _ := NewManager(config)
	defer mgr.Close()
	
	var receivedEvents []EventType
	var mu sync.RWMutex
	
	mgr.GetEventBus().Subscribe(EventListenerFunc(func(event Event) {
		mu.Lock()
		receivedEvents = append(receivedEvents, event.Type())
		mu.Unlock()
	}))
	
	// 执行成功
	req := &Request{
		Resource: "test",
		Execute: func(ctx context.Context) (interface{}, error) {
			return "ok", nil
		},
	}
	mgr.Execute(context.Background(), req)
	
	time.Sleep(50 * time.Millisecond)
	
	mu.RLock()
	defer mu.RUnlock()
	
	assert.Contains(t, receivedEvents, EventCallSuccess)
}

// TestCircuitBreaker_MetricsCollection 测试指标采集
func TestCircuitBreaker_MetricsCollection(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	
	mgr, _ := NewManager(config)
	defer mgr.Close()
	
	// 执行一些请求
	for i := 0; i < 10; i++ {
		req := &Request{
			Resource: "test",
			Execute: func(ctx context.Context) (interface{}, error) {
				time.Sleep(10 * time.Millisecond)
				if i < 7 {
					return "ok", nil
				}
				return nil, errors.New("error")
			},
		}
		mgr.Execute(context.Background(), req)
	}
	
	breaker := mgr.GetBreaker("test")
	metrics := breaker.GetMetrics()
	
	assert.Equal(t, int64(10), metrics.TotalRequests)
	assert.Equal(t, int64(7), metrics.Successes)
	assert.Equal(t, int64(3), metrics.Failures)
}

// TestCircuitBreaker_Reset 测试重置
func TestCircuitBreaker_Reset(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Default.ErrorRateThreshold = 0.5
	config.Default.MinRequests = 5
	
	mgr, _ := NewManager(config)
	defer mgr.Close()
	
	breaker := mgr.getOrCreateBreaker("test")
	
	// 触发熔断
	for i := 0; i < 10; i++ {
		req := &Request{
			Resource: "test",
			Execute: func(ctx context.Context) (interface{}, error) {
				return nil, errors.New("error")
			},
		}
		mgr.Execute(context.Background(), req)
	}
	
	assert.Equal(t, StateOpen, breaker.GetState())
	
	// 重置
	breaker.Reset()
	
	assert.Equal(t, StateClosed, breaker.GetState())
	metrics := breaker.GetMetrics()
	assert.Equal(t, int64(0), metrics.TotalRequests)
}

// TestCircuitBreaker_ConcurrentAccess 测试并发访问
func TestCircuitBreaker_ConcurrentAccess(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	
	mgr, _ := NewManager(config)
	defer mgr.Close()
	
	var successCount int32
	var errorCount int32
	
	// 并发执行
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			
			req := &Request{
				Resource: "test",
				Execute: func(ctx context.Context) (interface{}, error) {
					if idx%3 == 0 {
						return nil, errors.New("error")
					}
					return "ok", nil
				},
			}
			
			_, err := mgr.Execute(context.Background(), req)
			if err != nil {
				atomic.AddInt32(&errorCount, 1)
			} else {
				atomic.AddInt32(&successCount, 1)
			}
		}(i)
	}
	
	wg.Wait()
	
	// 验证计数
	total := atomic.LoadInt32(&successCount) + atomic.LoadInt32(&errorCount)
	assert.True(t, total > 0)
}

// TestCircuitBreaker_MultipleResources 测试多资源
func TestCircuitBreaker_MultipleResources(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	
	mgr, _ := NewManager(config)
	defer mgr.Close()
	
	// 为不同资源执行请求
	resources := []string{"service-a", "service-b", "service-c"}
	
	for _, resource := range resources {
		req := &Request{
			Resource: resource,
			Execute: func(ctx context.Context) (interface{}, error) {
				return "ok", nil
			},
		}
		mgr.Execute(context.Background(), req)
	}
	
	// 验证每个资源都有自己的熔断器
	for _, resource := range resources {
		state := mgr.GetState(resource)
		assert.Equal(t, StateClosed, state)
	}
}

// TestCircuitBreaker_Timeout 测试超时处理
func TestCircuitBreaker_Timeout(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	
	mgr, _ := NewManager(config)
	defer mgr.Close()
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	
	req := &Request{
		Resource: "test",
		Execute: func(ctx context.Context) (interface{}, error) {
			<-ctx.Done() // 等待context超时
			return nil, ctx.Err()
		},
	}
	
	_, err := mgr.Execute(ctx, req)
	
	assert.Error(t, err)
	
	// 给指标采集器一些时间处理
	time.Sleep(50 * time.Millisecond)
	
	// 验证指标记录了超时
	metrics := mgr.GetMetrics("test")
	assert.Equal(t, int64(1), metrics.Timeouts)
}
