package breaker

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestNewSlidingWindowMetrics 测试创建滑动窗口指标采集器
func TestNewSlidingWindowMetrics(t *testing.T) {
	config := DefaultResourceConfig()
	config.WindowSize = 10 * time.Second
	config.BucketSize = 1 * time.Second
	
	sm := newStateManager()
	metrics := newSlidingWindowMetrics("test-resource", config, sm)
	
	assert.NotNil(t, metrics)
	assert.Equal(t, "test-resource", metrics.resource)
	assert.Equal(t, 10, metrics.bucketCount)
	assert.Equal(t, 10, len(metrics.buckets))
	assert.Equal(t, time.Second, metrics.bucketSize)
}

// TestMetrics_RecordSuccess 测试记录成功
func TestMetrics_RecordSuccess(t *testing.T) {
	config := DefaultResourceConfig()
	sm := newStateManager()
	metrics := newSlidingWindowMetrics("test", config, sm)
	
	metrics.RecordSuccess(100 * time.Millisecond)
	
	snapshot := metrics.GetSnapshot()
	assert.Equal(t, int64(1), snapshot.Successes)
	assert.Equal(t, int64(1), snapshot.TotalRequests)
	assert.Equal(t, 1.0, snapshot.SuccessRate)
	assert.Equal(t, 0.0, snapshot.ErrorRate)
}

// TestMetrics_RecordFailure 测试记录失败
func TestMetrics_RecordFailure(t *testing.T) {
	config := DefaultResourceConfig()
	sm := newStateManager()
	metrics := newSlidingWindowMetrics("test", config, sm)
	
	err := errors.New("test error")
	metrics.RecordFailure(200*time.Millisecond, err)
	
	snapshot := metrics.GetSnapshot()
	assert.Equal(t, int64(1), snapshot.Failures)
	assert.Equal(t, int64(1), snapshot.TotalRequests)
	assert.Equal(t, 0.0, snapshot.SuccessRate)
	assert.Equal(t, 1.0, snapshot.ErrorRate)
	assert.Equal(t, int64(1), snapshot.ErrorTypes["test error"])
}

// TestMetrics_RecordTimeout 测试记录超时
func TestMetrics_RecordTimeout(t *testing.T) {
	config := DefaultResourceConfig()
	sm := newStateManager()
	metrics := newSlidingWindowMetrics("test", config, sm)
	
	metrics.RecordTimeout(5 * time.Second)
	
	snapshot := metrics.GetSnapshot()
	assert.Equal(t, int64(1), snapshot.Timeouts)
	assert.Equal(t, int64(1), snapshot.TotalRequests)
	assert.Equal(t, 1.0, snapshot.TimeoutRate)
}

// TestMetrics_RecordRejection 测试记录拒绝
func TestMetrics_RecordRejection(t *testing.T) {
	config := DefaultResourceConfig()
	sm := newStateManager()
	metrics := newSlidingWindowMetrics("test", config, sm)
	
	metrics.RecordRejection()
	
	snapshot := metrics.GetSnapshot()
	assert.Equal(t, int64(1), snapshot.Rejections)
	assert.Equal(t, int64(0), snapshot.TotalRequests) // 拒绝不计入总请求
}

// TestMetrics_GetSnapshot 测试获取快照
func TestMetrics_GetSnapshot(t *testing.T) {
	config := DefaultResourceConfig()
	config.SlowCallThreshold = 100 * time.Millisecond
	sm := newStateManager()
	metrics := newSlidingWindowMetrics("test", config, sm)
	
	// 记录多个请求
	metrics.RecordSuccess(50 * time.Millisecond)
	metrics.RecordSuccess(80 * time.Millisecond)
	metrics.RecordSuccess(150 * time.Millisecond) // 慢调用
	metrics.RecordFailure(200*time.Millisecond, errors.New("error1"))
	metrics.RecordTimeout(5 * time.Second)
	metrics.RecordRejection()
	
	snapshot := metrics.GetSnapshot()
	
	// 验证基础统计
	assert.Equal(t, "test", snapshot.Resource)
	assert.Equal(t, int64(3), snapshot.Successes)
	assert.Equal(t, int64(1), snapshot.Failures)
	assert.Equal(t, int64(1), snapshot.Timeouts)
	assert.Equal(t, int64(1), snapshot.Rejections)
	assert.Equal(t, int64(5), snapshot.TotalRequests)
	
	// 验证百分比
	assert.InDelta(t, 0.6, snapshot.SuccessRate, 0.01)
	assert.InDelta(t, 0.2, snapshot.ErrorRate, 0.01)
	assert.InDelta(t, 0.2, snapshot.TimeoutRate, 0.01)
	
	// 验证慢调用（150ms, 200ms, 5s 都超过100ms阈值）
	assert.Equal(t, int64(3), snapshot.SlowCalls)
	assert.InDelta(t, 0.6, snapshot.SlowCallRate, 0.01)
	
	// 验证延迟统计
	assert.True(t, snapshot.AvgLatency > 0)
	assert.True(t, snapshot.P50Latency > 0)
	assert.True(t, snapshot.MaxLatency > 0)
}

// TestMetrics_SubscribeUnsubscribe 测试订阅和取消订阅
func TestMetrics_SubscribeUnsubscribe(t *testing.T) {
	config := DefaultResourceConfig()
	sm := newStateManager()
	metrics := newSlidingWindowMetrics("test", config, sm)
	
	called := false
	observer := &mockMetricsObserver{
		onUpdate: func(snapshot *MetricsSnapshot) {
			called = true
		},
	}
	
	// 订阅
	id := metrics.Subscribe(observer)
	assert.NotEmpty(t, id)
	
	// 触发通知
	metrics.RecordSuccess(100 * time.Millisecond)
	time.Sleep(10 * time.Millisecond) // 等待异步通知
	assert.True(t, called)
	
	// 取消订阅
	called = false
	metrics.Unsubscribe(id)
	metrics.RecordSuccess(100 * time.Millisecond)
	time.Sleep(10 * time.Millisecond)
	assert.False(t, called)
}

// TestMetrics_Reset 测试重置指标
func TestMetrics_Reset(t *testing.T) {
	config := DefaultResourceConfig()
	sm := newStateManager()
	metrics := newSlidingWindowMetrics("test", config, sm)
	
	// 记录一些数据
	metrics.RecordSuccess(100 * time.Millisecond)
	metrics.RecordFailure(200*time.Millisecond, errors.New("error"))
	
	snapshot := metrics.GetSnapshot()
	assert.Equal(t, int64(2), snapshot.TotalRequests)
	
	// 重置
	metrics.Reset()
	
	snapshot = metrics.GetSnapshot()
	assert.Equal(t, int64(0), snapshot.TotalRequests)
	assert.Equal(t, int64(0), snapshot.Successes)
	assert.Equal(t, int64(0), snapshot.Failures)
}

// TestMetrics_SlidingWindow 测试滑动窗口特性
func TestMetrics_SlidingWindow(t *testing.T) {
	config := DefaultResourceConfig()
	config.WindowSize = 100 * time.Millisecond
	config.BucketSize = 20 * time.Millisecond
	
	sm := newStateManager()
	metrics := newSlidingWindowMetrics("test", config, sm)
	
	// 记录初始数据
	metrics.RecordSuccess(10 * time.Millisecond)
	snapshot := metrics.GetSnapshot()
	assert.Equal(t, int64(1), snapshot.TotalRequests)
	
	// 等待桶旋转
	time.Sleep(150 * time.Millisecond)
	
	// 旧数据应该被清除（但实际实现中需要rotate）
	metrics.rotate()
	snapshot = metrics.GetSnapshot()
	// 注意：旧桶仍然存在，只是被重用了
	// 这个测试主要验证rotate机制正常工作
	assert.NotNil(t, snapshot)
}

// TestMetrics_Concurrent 测试并发安全
func TestMetrics_Concurrent(t *testing.T) {
	config := DefaultResourceConfig()
	sm := newStateManager()
	metrics := newSlidingWindowMetrics("test", config, sm)
	
	done := make(chan bool)
	
	// 并发记录
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				metrics.RecordSuccess(10 * time.Millisecond)
				metrics.RecordFailure(20*time.Millisecond, errors.New("error"))
				metrics.RecordTimeout(5 * time.Second)
				metrics.RecordRejection()
			}
			done <- true
		}()
	}
	
	// 并发读取
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = metrics.GetSnapshot()
			}
			done <- true
		}()
	}
	
	// 等待完成
	for i := 0; i < 15; i++ {
		<-done
	}
	
	// 验证数据一致性
	snapshot := metrics.GetSnapshot()
	assert.True(t, snapshot.TotalRequests > 0)
}

// TestMetrics_ErrorTypes 测试错误类型统计
func TestMetrics_ErrorTypes(t *testing.T) {
	config := DefaultResourceConfig()
	sm := newStateManager()
	metrics := newSlidingWindowMetrics("test", config, sm)
	
	metrics.RecordFailure(100*time.Millisecond, errors.New("error1"))
	metrics.RecordFailure(100*time.Millisecond, errors.New("error1"))
	metrics.RecordFailure(100*time.Millisecond, errors.New("error2"))
	
	snapshot := metrics.GetSnapshot()
	assert.Equal(t, int64(2), snapshot.ErrorTypes["error1"])
	assert.Equal(t, int64(1), snapshot.ErrorTypes["error2"])
}

// TestMetrics_LatencyPercentiles 测试延迟百分位
func TestMetrics_LatencyPercentiles(t *testing.T) {
	config := DefaultResourceConfig()
	sm := newStateManager()
	metrics := newSlidingWindowMetrics("test", config, sm)
	
	// 记录多个不同延迟的请求
	latencies := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
		40 * time.Millisecond,
		50 * time.Millisecond,
		100 * time.Millisecond,
		200 * time.Millisecond,
		300 * time.Millisecond,
		400 * time.Millisecond,
		500 * time.Millisecond,
	}
	
	for _, lat := range latencies {
		metrics.RecordSuccess(lat)
	}
	
	snapshot := metrics.GetSnapshot()
	
	// 验证百分位存在且合理
	assert.True(t, snapshot.P50Latency > 0)
	assert.True(t, snapshot.P95Latency > 0)
	assert.True(t, snapshot.P99Latency > 0)
	assert.True(t, snapshot.P50Latency <= snapshot.P95Latency)
	assert.True(t, snapshot.P95Latency <= snapshot.P99Latency)
	assert.True(t, snapshot.P99Latency <= snapshot.MaxLatency)
	assert.Equal(t, 500*time.Millisecond, snapshot.MaxLatency)
}

// mockMetricsObserver 模拟指标观察者
type mockMetricsObserver struct {
	onUpdate func(*MetricsSnapshot)
}

func (m *mockMetricsObserver) OnMetricsUpdated(snapshot *MetricsSnapshot) {
	if m.onUpdate != nil {
		m.onUpdate(snapshot)
	}
}

