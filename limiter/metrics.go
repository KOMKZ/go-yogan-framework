package limiter

import (
	"sync"
	"sync/atomic"
	"time"
)

// MetricsSnapshot metric snapshot
type MetricsSnapshot struct {
	Resource       string
	Algorithm      string
	TotalRequests  int64
	Allowed        int64
	Rejected       int64
	CurrentValue   int64  // Current value (concurrency count/token count/request count)
	Limit          int64  // Limit value
	Remaining      int64  // remaining quota
	RejectRate     float64 // rejection rate
	LastResetAt    time.Time
}

// MetricsCollector metric collector interface
type MetricsCollector interface {
	// RecordAllowed records allowed requests
	RecordAllowed(remaining int64)

	// RecordRejected Request for record rejected
	RecordRejected(reason string)

	// GetSnapshot Retrieve metric snapshot
	GetSnapshot() *MetricsSnapshot

	// Reset metrics
	Reset()
}

// metricsCollector metric collector implementation
type metricsCollector struct {
	resource      string
	algorithm     string
	totalRequests int64
	allowed       int64
	rejected      int64
	lastResetAt   time.Time
	mu            sync.RWMutex
}

// Create metric collector
func NewMetricsCollector(resource string, algorithm string) MetricsCollector {
	return &metricsCollector{
		resource:    resource,
		algorithm:   algorithm,
		lastResetAt: time.Now(),
	}
}

// RecordAllowed records allowed requests
func (m *metricsCollector) RecordAllowed(remaining int64) {
	atomic.AddInt64(&m.totalRequests, 1)
	atomic.AddInt64(&m.allowed, 1)
}

// RecordRejected Request rejected
func (m *metricsCollector) RecordRejected(reason string) {
	atomic.AddInt64(&m.totalRequests, 1)
	atomic.AddInt64(&m.rejected, 1)
}

// GetSnapshot retrieves metric snapshot
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

// Reset metrics
func (m *metricsCollector) Reset() {
	atomic.StoreInt64(&m.totalRequests, 0)
	atomic.StoreInt64(&m.allowed, 0)
	atomic.StoreInt64(&m.rejected, 0)

	m.mu.Lock()
	m.lastResetAt = time.Now()
	m.mu.Unlock()
}

