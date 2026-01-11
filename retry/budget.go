package retry

import (
	"sync"
	"time"
)

// BudgetManager 重试预算管理器
// 用于限制重试流量，防止重试放大导致的流量激增
type BudgetManager struct {
	ratio  float64       // 预算比例（如 0.1 = 10%）
	window time.Duration // 统计窗口（如 1 分钟）
	
	mu      sync.Mutex
	requests int64 // 原始请求数
	retries  int64 // 重试请求数
	
	windowStart time.Time // 窗口开始时间
}

// NewBudgetManager 创建重试预算管理器
// ratio: 预算比例（0.0 - 1.0），如 0.1 表示重试请求不超过原始请求的 10%
// window: 统计窗口，如 time.Minute
func NewBudgetManager(ratio float64, window time.Duration) *BudgetManager {
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1.0 {
		ratio = 1.0
	}
	if window <= 0 {
		window = time.Minute
	}
	
	return &BudgetManager{
		ratio:       ratio,
		window:      window,
		windowStart: time.Now(),
	}
}

// Allow 检查是否允许重试（预算检查）
// 返回 true 表示允许重试，false 表示预算耗尽
func (b *BudgetManager) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	// 检查是否需要重置窗口
	b.maybeResetWindow()
	
	// 计算当前预算上限
	maxRetries := int64(float64(b.requests) * b.ratio)
	
	// 检查是否还有预算
	return b.retries < maxRetries
}

// Record 记录请求结果（更新预算统计）
// success: 请求是否成功
func (b *BudgetManager) Record(success bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	// 检查是否需要重置窗口
	b.maybeResetWindow()
	
	// 更新统计
	b.requests++
	if !success {
		b.retries++
	}
}

// GetStats 获取预算统计信息
func (b *BudgetManager) GetStats() BudgetStats {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	// 检查是否需要重置窗口
	b.maybeResetWindow()
	
	maxRetries := int64(float64(b.requests) * b.ratio)
	remaining := maxRetries - b.retries
	if remaining < 0 {
		remaining = 0
	}
	
	return BudgetStats{
		Requests:    b.requests,
		Retries:     b.retries,
		MaxRetries:  maxRetries,
		Remaining:   remaining,
		Ratio:       b.ratio,
		UsageRatio:  b.calculateUsageRatio(),
		WindowStart: b.windowStart,
		WindowEnd:   b.windowStart.Add(b.window),
	}
}

// Reset 重置预算统计
func (b *BudgetManager) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	b.requests = 0
	b.retries = 0
	b.windowStart = time.Now()
}

// maybeResetWindow 检查是否需要重置窗口（内部方法，调用前需加锁）
func (b *BudgetManager) maybeResetWindow() {
	now := time.Now()
	if now.Sub(b.windowStart) >= b.window {
		// 窗口过期，重置统计
		b.requests = 0
		b.retries = 0
		b.windowStart = now
	}
}

// calculateUsageRatio 计算预算使用率（内部方法，调用前需加锁）
func (b *BudgetManager) calculateUsageRatio() float64 {
	if b.requests == 0 {
		return 0
	}
	
	maxRetries := float64(b.requests) * b.ratio
	if maxRetries == 0 {
		return 0
	}
	
	return float64(b.retries) / maxRetries
}

// BudgetStats 预算统计信息
type BudgetStats struct {
	Requests    int64         // 原始请求数
	Retries     int64         // 重试请求数
	MaxRetries  int64         // 最大允许重试数
	Remaining   int64         // 剩余预算
	Ratio       float64       // 预算比例
	UsageRatio  float64       // 预算使用率（0.0 - 1.0）
	WindowStart time.Time     // 窗口开始时间
	WindowEnd   time.Time     // 窗口结束时间
}

// IsExhausted 判断预算是否耗尽
func (s *BudgetStats) IsExhausted() bool {
	return s.Remaining <= 0
}

// UsagePercent 返回预算使用百分比（0 - 100）
func (s *BudgetStats) UsagePercent() float64 {
	return s.UsageRatio * 100
}

