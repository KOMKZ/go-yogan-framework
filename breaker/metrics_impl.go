package breaker

import (
	"sort"
	"sync"
	"time"
)

// slidingWindowMetrics 滑动窗口指标采集器
type slidingWindowMetrics struct {
	resource      string
	config        ResourceConfig
	stateMgr      *stateManager
	
	// 时间窗口（环形桶）
	buckets       []*bucket
	bucketCount   int
	bucketSize    time.Duration
	currentIdx    int
	lastRotate    time.Time
	
	// 观察者
	observers     map[ObserverID]MetricsObserver
	observerMu    sync.RWMutex
	
	mu            sync.RWMutex
}

// bucket 时间桶
type bucket struct {
	startTime     time.Time
	successes     int64
	failures      int64
	timeouts      int64
	rejections    int64
	latencies     []time.Duration
	errorTypes    map[string]int64
	mu            sync.RWMutex
}

// newBucket 创建新桶
func newBucket(startTime time.Time) *bucket {
	return &bucket{
		startTime:  startTime,
		latencies:  make([]time.Duration, 0, 100),
		errorTypes: make(map[string]int64),
	}
}

// newSlidingWindowMetrics 创建滑动窗口指标采集器
func newSlidingWindowMetrics(resource string, config ResourceConfig, stateMgr *stateManager) *slidingWindowMetrics {
	bucketCount := int(config.WindowSize / config.BucketSize)
	buckets := make([]*bucket, bucketCount)
	now := time.Now()
	
	for i := 0; i < bucketCount; i++ {
		buckets[i] = newBucket(now.Add(-time.Duration(bucketCount-i) * config.BucketSize))
	}
	
	return &slidingWindowMetrics{
		resource:    resource,
		config:      config,
		stateMgr:    stateMgr,
		buckets:     buckets,
		bucketCount: bucketCount,
		bucketSize:  config.BucketSize,
		lastRotate:  now,
		observers:   make(map[ObserverID]MetricsObserver),
	}
}

// RecordSuccess 记录成功
func (m *slidingWindowMetrics) RecordSuccess(duration time.Duration) {
	m.rotate()
	
	bucket := m.getCurrentBucket()
	bucket.mu.Lock()
	bucket.successes++
	bucket.latencies = append(bucket.latencies, duration)
	bucket.mu.Unlock()
	
	m.notifyObservers()
}

// RecordFailure 记录失败
func (m *slidingWindowMetrics) RecordFailure(duration time.Duration, err error) {
	m.rotate()
	
	bucket := m.getCurrentBucket()
	bucket.mu.Lock()
	bucket.failures++
	bucket.latencies = append(bucket.latencies, duration)
	
	if err != nil {
		errType := err.Error()
		bucket.errorTypes[errType]++
	}
	bucket.mu.Unlock()
	
	m.notifyObservers()
}

// RecordTimeout 记录超时
func (m *slidingWindowMetrics) RecordTimeout(duration time.Duration) {
	m.rotate()
	
	bucket := m.getCurrentBucket()
	bucket.mu.Lock()
	bucket.timeouts++
	bucket.latencies = append(bucket.latencies, duration)
	bucket.mu.Unlock()
	
	m.notifyObservers()
}

// RecordRejection 记录拒绝
func (m *slidingWindowMetrics) RecordRejection() {
	m.rotate()
	
	bucket := m.getCurrentBucket()
	bucket.mu.Lock()
	bucket.rejections++
	bucket.mu.Unlock()
	
	m.notifyObservers()
}

// GetSnapshot 获取当前快照
func (m *slidingWindowMetrics) GetSnapshot() *MetricsSnapshot {
	m.rotate()
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var (
		totalRequests int64
		successes     int64
		failures      int64
		timeouts      int64
		rejections    int64
		allLatencies  []time.Duration
		errorTypes    = make(map[string]int64)
	)
	
	// 聚合所有桶的数据
	windowStart := time.Now().Add(-m.config.WindowSize)
	windowEnd := time.Now()
	
	for _, bucket := range m.buckets {
		bucket.mu.RLock()
		successes += bucket.successes
		failures += bucket.failures
		timeouts += bucket.timeouts
		rejections += bucket.rejections
		allLatencies = append(allLatencies, bucket.latencies...)
		
		for errType, count := range bucket.errorTypes {
			errorTypes[errType] += count
		}
		bucket.mu.RUnlock()
	}
	
	totalRequests = successes + failures + timeouts
	
	// 计算百分比
	var successRate, errorRate, timeoutRate float64
	if totalRequests > 0 {
		successRate = float64(successes) / float64(totalRequests)
		errorRate = float64(failures) / float64(totalRequests)
		timeoutRate = float64(timeouts) / float64(totalRequests)
	}
	
	// 计算延迟统计
	var avgLatency, p50, p95, p99, maxLatency time.Duration
	var slowCalls int64
	var slowCallRate float64
	
	if len(allLatencies) > 0 {
		sort.Slice(allLatencies, func(i, j int) bool {
			return allLatencies[i] < allLatencies[j]
		})
		
		// 平均延迟
		var total time.Duration
		for _, lat := range allLatencies {
			total += lat
			if lat >= m.config.SlowCallThreshold {
				slowCalls++
			}
		}
		avgLatency = total / time.Duration(len(allLatencies))
		
		// 百分位
		p50 = allLatencies[len(allLatencies)*50/100]
		p95 = allLatencies[len(allLatencies)*95/100]
		p99 = allLatencies[len(allLatencies)*99/100]
		maxLatency = allLatencies[len(allLatencies)-1]
		
		// 慢调用率
		if totalRequests > 0 {
			slowCallRate = float64(slowCalls) / float64(totalRequests)
		}
	}
	
	return &MetricsSnapshot{
		Resource:      m.resource,
		State:         m.stateMgr.GetState(),
		WindowStart:   windowStart,
		WindowEnd:     windowEnd,
		TotalRequests: totalRequests,
		Successes:     successes,
		Failures:      failures,
		Timeouts:      timeouts,
		Rejections:    rejections,
		SuccessRate:   successRate,
		ErrorRate:     errorRate,
		TimeoutRate:   timeoutRate,
		AvgLatency:    avgLatency,
		P50Latency:    p50,
		P95Latency:    p95,
		P99Latency:    p99,
		MaxLatency:    maxLatency,
		SlowCalls:     slowCalls,
		SlowCallRate:  slowCallRate,
		ErrorTypes:    errorTypes,
	}
}

// Subscribe 订阅实时指标
func (m *slidingWindowMetrics) Subscribe(observer MetricsObserver) ObserverID {
	m.observerMu.Lock()
	defer m.observerMu.Unlock()
	
	id := ObserverID(time.Now().Format("20060102150405.000000"))
	m.observers[id] = observer
	return id
}

// Unsubscribe 取消订阅
func (m *slidingWindowMetrics) Unsubscribe(id ObserverID) {
	m.observerMu.Lock()
	defer m.observerMu.Unlock()
	
	delete(m.observers, id)
}

// Reset 重置指标
func (m *slidingWindowMetrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	now := time.Now()
	for i := 0; i < m.bucketCount; i++ {
		m.buckets[i] = newBucket(now.Add(-time.Duration(m.bucketCount-i) * m.bucketSize))
	}
	m.lastRotate = now
	m.currentIdx = 0
}

// rotate 旋转桶（如果需要）
func (m *slidingWindowMetrics) rotate() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	now := time.Now()
	elapsed := now.Sub(m.lastRotate)
	
	// 计算需要旋转的桶数
	rotations := int(elapsed / m.bucketSize)
	if rotations == 0 {
		return
	}
	
	// 限制最大旋转数（避免超出桶数量）
	if rotations > m.bucketCount {
		rotations = m.bucketCount
	}
	
	// 旋转桶
	for i := 0; i < rotations; i++ {
		m.currentIdx = (m.currentIdx + 1) % m.bucketCount
		m.buckets[m.currentIdx] = newBucket(now)
	}
	
	m.lastRotate = now
}

// getCurrentBucket 获取当前桶
func (m *slidingWindowMetrics) getCurrentBucket() *bucket {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.buckets[m.currentIdx]
}

// notifyObservers 通知所有观察者
func (m *slidingWindowMetrics) notifyObservers() {
	m.observerMu.RLock()
	observers := make([]MetricsObserver, 0, len(m.observers))
	for _, obs := range m.observers {
		observers = append(observers, obs)
	}
	m.observerMu.RUnlock()
	
	if len(observers) == 0 {
		return
	}
	
	snapshot := m.GetSnapshot()
	for _, obs := range observers {
		go obs.OnMetricsUpdated(snapshot)
	}
}

