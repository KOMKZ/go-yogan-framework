package limiter

import (
	"sync"
	"sync/atomic"
	"time"
)

// MetricsSnapshot 指标快照
type MetricsSnapshot struct {
	Resource       string
	Algorithm      string
	TotalRequests  int64
	Allowed        int64
	Rejected       int64
	CurrentValue   int64  // 当前值（并发数/令牌数/请求数）
	Limit          int64  // 限制值
	Remaining      int64  // 剩余配额
	RejectRate     float64 // 拒绝率
	LastResetAt    time.Time
}

// MetricsCollector 指标采集器接口
type MetricsCollector interface {
	// RecordAllowed 记录允许的请求
	RecordAllowed(remaining int64)

	// RecordRejected 记录被拒绝的请求
	RecordRejected(reason string)

	// GetSnapshot 获取指标快照
	GetSnapshot() *MetricsSnapshot

	// Reset 重置指标
	Reset()
}

// metricsCollector 指标采集器实现
type metricsCollector struct {
	resource      string
	algorithm     string
	totalRequests int64
	allowed       int64
	rejected      int64
	lastResetAt   time.Time
	mu            sync.RWMutex
}

// NewMetricsCollector 创建指标采集器
func NewMetricsCollector(resource string, algorithm string) MetricsCollector {
	return &metricsCollector{
		resource:    resource,
		algorithm:   algorithm,
		lastResetAt: time.Now(),
	}
}

// RecordAllowed 记录允许的请求
func (m *metricsCollector) RecordAllowed(remaining int64) {
	atomic.AddInt64(&m.totalRequests, 1)
	atomic.AddInt64(&m.allowed, 1)
}

// RecordRejected 记录被拒绝的请求
func (m *metricsCollector) RecordRejected(reason string) {
	atomic.AddInt64(&m.totalRequests, 1)
	atomic.AddInt64(&m.rejected, 1)
}

// GetSnapshot 获取指标快照
func (m *metricsCollector) GetSnapshot() *MetricsSnapshot {
	total := atomic.LoadInt64(&m.totalRequests)
	allowed := atomic.LoadInt64(&m.allowed)
	rejected := atomic.LoadInt64(&m.rejected)

	var rejectRate float64
	if total > 0 {
		rejectRate = float64(rejected) / float64(total)
	}

	m.mu.RLock()
	lastResetAt := m.lastResetAt
	m.mu.RUnlock()

	return &MetricsSnapshot{
		Resource:      m.resource,
		Algorithm:     m.algorithm,
		TotalRequests: total,
		Allowed:       allowed,
		Rejected:      rejected,
		RejectRate:    rejectRate,
		LastResetAt:   lastResetAt,
	}
}

// Reset 重置指标
func (m *metricsCollector) Reset() {
	atomic.StoreInt64(&m.totalRequests, 0)
	atomic.StoreInt64(&m.allowed, 0)
	atomic.StoreInt64(&m.rejected, 0)

	m.mu.Lock()
	m.lastResetAt = time.Now()
	m.mu.Unlock()
}

