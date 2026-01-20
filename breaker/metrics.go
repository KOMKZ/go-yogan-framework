package breaker

import (
	"time"
)

// MetricsCollector metric collector interface
type MetricsCollector interface {
	// RecordSuccess Recording successful
	RecordSuccess(duration time.Duration)
	
	// RecordFailure log failure
	RecordFailure(duration time.Duration, err error)
	
	// RecordTimeout recording timeout
	RecordTimeout(duration time.Duration)
	
	// RecordRejection record rejection
	RecordRejection()
	
	// GetSnapshot Get current snapshot
	GetSnapshot() *MetricsSnapshot
	
	// Subscribe to real-time metrics
	Subscribe(observer MetricsObserver) ObserverID
	
	// Unsubscribe from subscription
	Unsubscribe(id ObserverID)
	
	// Reset metrics
	Reset()
}

// MetricsSnapshot metric snapshot (accessible at the application layer)
type MetricsSnapshot struct {
	Resource      string
	State         State
	WindowStart   time.Time
	WindowEnd     time.Time
	
	// count statistics
	TotalRequests int64
	Successes     int64
	Failures      int64
	Timeouts      int64
	Rejections    int64
	
	// Percentage
	SuccessRate   float64 // success rate
	ErrorRate     float64 // error rate
	TimeoutRate   float64 // timeout rate
	
	// delayed statistics
	AvgLatency    time.Duration
	P50Latency    time.Duration
	P95Latency    time.Duration
	P99Latency    time.Duration
	MaxLatency    time.Duration
	
	// slow call statistics
	SlowCalls     int64
	SlowCallRate  float64
	
	// Error distribution (optional)
	ErrorTypes    map[string]int64
}

// MetricsObserver metric observer (application layer implementation)
type MetricsObserver interface {
	OnMetricsUpdated(snapshot *MetricsSnapshot)
}

// ObserverID observer ID
type ObserverID string

