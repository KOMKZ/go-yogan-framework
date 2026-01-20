package breaker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestErrorRateStrategy test error rate strategy
func TestErrorRateStrategy(t *testing.T) {
	strategy := &errorRateStrategy{}
	config := DefaultResourceConfig()
	config.MinRequests = 10
	config.ErrorRateThreshold = 0.5
	
	assert.Equal(t, "error_rate", strategy.Name())
	
	t.Run("请求数不足不触发", func(t *testing.T) {
		snapshot := &MetricsSnapshot{
			TotalRequests: 5,
			ErrorRate:     0.8,
		}
		assert.False(t, strategy.ShouldOpen(snapshot, config))
	})
	
	t.Run("错误率未达阈值不触发", func(t *testing.T) {
		snapshot := &MetricsSnapshot{
			TotalRequests: 20,
			ErrorRate:     0.3,
		}
		assert.False(t, strategy.ShouldOpen(snapshot, config))
	})
	
	t.Run("错误率达到阈值触发熔断", func(t *testing.T) {
		snapshot := &MetricsSnapshot{
			TotalRequests: 20,
			ErrorRate:     0.6,
		}
		assert.True(t, strategy.ShouldOpen(snapshot, config))
	})
}

// TestSlowCallRateStrategy test slow call rate strategy
func TestSlowCallRateStrategy(t *testing.T) {
	strategy := &slowCallRateStrategy{}
	config := DefaultResourceConfig()
	config.MinRequests = 10
	config.SlowRateThreshold = 0.3
	
	assert.Equal(t, "slow_call_rate", strategy.Name())
	
	t.Run("请求数不足不触发", func(t *testing.T) {
		snapshot := &MetricsSnapshot{
			TotalRequests: 5,
			SlowCallRate:  0.8,
		}
		assert.False(t, strategy.ShouldOpen(snapshot, config))
	})
	
	t.Run("慢调用率未达阈值不触发", func(t *testing.T) {
		snapshot := &MetricsSnapshot{
			TotalRequests: 20,
			SlowCallRate:  0.2,
		}
		assert.False(t, strategy.ShouldOpen(snapshot, config))
	})
	
	t.Run("慢调用率达到阈值触发熔断", func(t *testing.T) {
		snapshot := &MetricsSnapshot{
			TotalRequests: 20,
			SlowCallRate:  0.4,
		}
		assert.True(t, strategy.ShouldOpen(snapshot, config))
	})
}

// TestConsecutiveFailuresStrategy test consecutive failures strategy
func TestConsecutiveFailuresStrategy(t *testing.T) {
	strategy := &consecutiveFailuresStrategy{}
	config := DefaultResourceConfig()
	config.ConsecutiveFailures = 5
	
	assert.Equal(t, "consecutive_failures", strategy.Name())
	
	t.Run("初始状态不触发", func(t *testing.T) {
		snapshot := &MetricsSnapshot{}
		assert.False(t, strategy.ShouldOpen(snapshot, config))
	})
	
	t.Run("连续失败未达阈值不触发", func(t *testing.T) {
		strategy := &consecutiveFailuresStrategy{}
		for i := 0; i < 3; i++ {
			strategy.RecordFailure()
		}
		
		snapshot := &MetricsSnapshot{}
		assert.False(t, strategy.ShouldOpen(snapshot, config))
	})
	
	t.Run("连续失败达到阈值触发熔断", func(t *testing.T) {
		strategy := &consecutiveFailuresStrategy{}
		for i := 0; i < 5; i++ {
			strategy.RecordFailure()
		}
		
		snapshot := &MetricsSnapshot{}
		assert.True(t, strategy.ShouldOpen(snapshot, config))
	})
	
	t.Run("成功后重置计数", func(t *testing.T) {
		strategy := &consecutiveFailuresStrategy{}
		for i := 0; i < 3; i++ {
			strategy.RecordFailure()
		}
		
		strategy.RecordSuccess()
		
		snapshot := &MetricsSnapshot{}
		assert.False(t, strategy.ShouldOpen(snapshot, config))
	})
}

// TestGetStrategyByName tests getting strategy by name
func TestGetStrategyByName(t *testing.T) {
	t.Run("获取错误率策略", func(t *testing.T) {
		strategy := GetStrategyByName("error_rate")
		assert.NotNil(t, strategy)
		assert.Equal(t, "error_rate", strategy.Name())
	})
	
	t.Run("获取慢调用率策略", func(t *testing.T) {
		strategy := GetStrategyByName("slow_call_rate")
		assert.NotNil(t, strategy)
		assert.Equal(t, "slow_call_rate", strategy.Name())
	})
	
	t.Run("获取连续失败策略", func(t *testing.T) {
		strategy := GetStrategyByName("consecutive_failures")
		assert.NotNil(t, strategy)
		assert.Equal(t, "consecutive_failures", strategy.Name())
	})
	
	t.Run("未知策略返回默认策略", func(t *testing.T) {
		strategy := GetStrategyByName("unknown")
		assert.NotNil(t, strategy)
		assert.Equal(t, "error_rate", strategy.Name())
	})
}

// TestStrategyIntegration test strategy integration
func TestStrategyIntegration(t *testing.T) {
	t.Run("错误率策略实际场景", func(t *testing.T) {
		config := DefaultResourceConfig()
		config.ErrorRateThreshold = 0.5
		config.MinRequests = 10
		
		sm := newStateManager()
		metrics := newSlidingWindowMetrics("test", config, sm)
		strategy := GetStrategyByName("error_rate")
		
		// Simulate 10 requests, 6 failures
		for i := 0; i < 4; i++ {
			metrics.RecordSuccess(100 * time.Millisecond)
		}
		for i := 0; i < 6; i++ {
			metrics.RecordFailure(100*time.Millisecond, nil)
		}
		
		snapshot := metrics.GetSnapshot()
		assert.True(t, strategy.ShouldOpen(snapshot, config))
	})
	
	t.Run("慢调用率策略实际场景", func(t *testing.T) {
		config := DefaultResourceConfig()
		config.SlowCallThreshold = 100 * time.Millisecond
		config.SlowRateThreshold = 0.3
		config.MinRequests = 10
		
		sm := newStateManager()
		metrics := newSlidingWindowMetrics("test", config, sm)
		strategy := GetStrategyByName("slow_call_rate")
		
		// Simulate 10 requests, 4 slow calls
		for i := 0; i < 6; i++ {
			metrics.RecordSuccess(50 * time.Millisecond)
		}
		for i := 0; i < 4; i++ {
			metrics.RecordSuccess(200 * time.Millisecond) // slow call
		}
		
		snapshot := metrics.GetSnapshot()
		assert.True(t, strategy.ShouldOpen(snapshot, config))
	})
}

// mock strategy (for testing)
type mockStrategy struct {
	name       string
	shouldOpen bool
}

func (s *mockStrategy) Name() string {
	return s.name
}

func (s *mockStrategy) ShouldOpen(snapshot *MetricsSnapshot, config ResourceConfig) bool {
	return s.shouldOpen
}

// TestMockStrategy test mock strategy
func TestMockStrategy(t *testing.T) {
	strategy := &mockStrategy{
		name:       "mock",
		shouldOpen: true,
	}
	
	assert.Equal(t, "mock", strategy.Name())
	assert.True(t, strategy.ShouldOpen(nil, ResourceConfig{}))
}

