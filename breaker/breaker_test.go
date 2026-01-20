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

// TestNewManager test create manager
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
		config.Default.MinRequests = -1 // invalid configuration
		
		mgr, err := NewManager(config)
		assert.Error(t, err)
		assert.Nil(t, mgr)
	})
}

// TestManager_Execute_Disabled Directly pass through when testing is not enabled
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

// TestManager_Execute_Success Execution successful
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
	
	// Verify that the circuit breaker is created
	state := mgr.GetState("test-service")
	assert.Equal(t, StateClosed, state)
}

// TestManager_Execute_Failure Execution failed
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

// TestManager_Execute_WithFallback test execution with fallback
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
	
	// Trigger enough failures to open the circuit breaker
	for i := 0; i < 10; i++ {
		mgr.Execute(context.Background(), req)
	}
	
	// After the circuit breaker trips, requests are rejected, and degradation should be executed
	result, err := mgr.Execute(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, "fallback result", result)
}

// TestCircuitBreaker_StateTransition Test state transition
func TestCircuitBreaker_StateTransition(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Default.ErrorRateThreshold = 0.5
	config.Default.MinRequests = 10
	config.Default.Timeout = 100 * time.Millisecond
	
	mgr, _ := NewManager(config)
	defer mgr.Close()
	
	// The initial state should be Closed
	breaker := mgr.getOrCreateBreaker("test")
	assert.Equal(t, StateClosed, breaker.GetState())
	
	// Simulate 10 requests, 6 failures
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
	
	// Circuit breaker should be triggered, status changes to Open
	assert.Equal(t, StateOpen, breaker.GetState())
	
	// After a timeout, the status should become HalfOpen
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

// TestCircuitBreaker_EventPublish test event publication
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
	
	// Execution successful
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

// TestCircuitBreaker_MetricsCollection test metrics collection
func TestCircuitBreaker_MetricsCollection(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	
	mgr, _ := NewManager(config)
	defer mgr.Close()
	
	// Execute some requests
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

// TestCircuitBreaker_Reset Test reset
func TestCircuitBreaker_Reset(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Default.ErrorRateThreshold = 0.5
	config.Default.MinRequests = 5
	
	mgr, _ := NewManager(config)
	defer mgr.Close()
	
	breaker := mgr.getOrCreateBreaker("test")
	
	// Trigger circuit breaker
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
	
	// Reset
	breaker.Reset()
	
	assert.Equal(t, StateClosed, breaker.GetState())
	metrics := breaker.GetMetrics()
	assert.Equal(t, int64(0), metrics.TotalRequests)
}

// TestConcurrentAccess Concurrent access test
func TestCircuitBreaker_ConcurrentAccess(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	
	mgr, _ := NewManager(config)
	defer mgr.Close()
	
	var successCount int32
	var errorCount int32
	
	// concurrent execution
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
	
	// Verify count
	total := atomic.LoadInt32(&successCount) + atomic.LoadInt32(&errorCount)
	assert.True(t, total > 0)
}

// TestCircuitBreaker_MultipleResources test multiple resources
func TestCircuitBreaker_MultipleResources(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	
	mgr, _ := NewManager(config)
	defer mgr.Close()
	
	// Make requests for different resources
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
	
	// Verify that each resource has its own circuit breaker
	for _, resource := range resources {
		state := mgr.GetState(resource)
		assert.Equal(t, StateClosed, state)
	}
}

// TestCircuitBreaker_Timeout Test timeout handling
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
			<-ctx.Done() // wait for context timeout
			return nil, ctx.Err()
		},
	}
	
	_, err := mgr.Execute(ctx, req)
	
	assert.Error(t, err)
	
	// Give the metric collector some time to process
	time.Sleep(50 * time.Millisecond)
	
	// The metric verifies if there was a timeout
	metrics := mgr.GetMetrics("test")
	assert.Equal(t, int64(1), metrics.Timeouts)
}
