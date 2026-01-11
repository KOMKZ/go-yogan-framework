package breaker

import (
	"time"
)

// MetricsCollector 指标采集器接口
type MetricsCollector interface {
	// RecordSuccess 记录成功
	RecordSuccess(duration time.Duration)
	
	// RecordFailure 记录失败
	RecordFailure(duration time.Duration, err error)
	
	// RecordTimeout 记录超时
	RecordTimeout(duration time.Duration)
	
	// RecordRejection 记录拒绝
	RecordRejection()
	
	// GetSnapshot 获取当前快照
	GetSnapshot() *MetricsSnapshot
	
	// Subscribe 订阅实时指标
	Subscribe(observer MetricsObserver) ObserverID
	
	// Unsubscribe 取消订阅
	Unsubscribe(id ObserverID)
	
	// Reset 重置指标
	Reset()
}

// MetricsSnapshot 指标快照（应用层可访问）
type MetricsSnapshot struct {
	Resource      string
	State         State
	WindowStart   time.Time
	WindowEnd     time.Time
	
	// 计数统计
	TotalRequests int64
	Successes     int64
	Failures      int64
	Timeouts      int64
	Rejections    int64
	
	// 百分比
	SuccessRate   float64 // 成功率
	ErrorRate     float64 // 错误率
	TimeoutRate   float64 // 超时率
	
	// 延迟统计
	AvgLatency    time.Duration
	P50Latency    time.Duration
	P95Latency    time.Duration
	P99Latency    time.Duration
	MaxLatency    time.Duration
	
	// 慢调用统计
	SlowCalls     int64
	SlowCallRate  float64
	
	// 错误分布（可选）
	ErrorTypes    map[string]int64
}

// MetricsObserver 指标观察者（应用层实现）
type MetricsObserver interface {
	OnMetricsUpdated(snapshot *MetricsSnapshot)
}

// ObserverID 观察者ID
type ObserverID string

