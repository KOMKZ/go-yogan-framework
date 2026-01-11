package health

import (
	"context"
	"sync"
	"time"
)

// Aggregator 健康检查聚合器
// 统一管理多个健康检查项
type Aggregator struct {
	checkers []Checker
	timeout  time.Duration
	mu       sync.RWMutex
	metadata map[string]interface{}
}

// NewAggregator 创建健康检查聚合器
func NewAggregator(timeout time.Duration) *Aggregator {
	if timeout <= 0 {
		timeout = 5 * time.Second // 默认 5 秒超时
	}
	return &Aggregator{
		checkers: make([]Checker, 0),
		timeout:  timeout,
		metadata: make(map[string]interface{}),
	}
}

// Register 注册健康检查项
func (a *Aggregator) Register(checker Checker) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.checkers = append(a.checkers, checker)
}

// SetMetadata 设置元数据
func (a *Aggregator) SetMetadata(key string, value interface{}) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.metadata[key] = value
}

// Check 执行所有健康检查
func (a *Aggregator) Check(ctx context.Context) *Response {
	start := time.Now()

	// 创建超时上下文
	checkCtx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()

	a.mu.RLock()
	checkers := make([]Checker, len(a.checkers))
	copy(checkers, a.checkers)
	metadata := make(map[string]interface{})
	for k, v := range a.metadata {
		metadata[k] = v
	}
	a.mu.RUnlock()

	// 并发执行所有检查
	results := make(chan CheckResult, len(checkers))
	for _, checker := range checkers {
		go func(c Checker) {
			results <- a.checkOne(checkCtx, c)
		}(checker)
	}

	// 收集结果
	checks := make(map[string]CheckResult)
	for i := 0; i < len(checkers); i++ {
		result := <-results
		checks[result.Name] = result
	}

	// 计算整体状态
	overallStatus := a.calculateOverallStatus(checks)

	return &Response{
		Status:    overallStatus,
		Timestamp: time.Now(),
		Duration:  time.Since(start),
		Checks:    checks,
		Metadata:  metadata,
	}
}

// checkOne 执行单个检查
func (a *Aggregator) checkOne(ctx context.Context, checker Checker) CheckResult {
	start := time.Now()
	result := CheckResult{
		Name:      checker.Name(),
		Timestamp: start,
	}

	err := checker.Check(ctx)
	result.Duration = time.Since(start)

	if err != nil {
		result.Status = StatusUnhealthy
		result.Error = err.Error()
		result.Message = "Health check failed"
	} else {
		result.Status = StatusHealthy
		result.Message = "OK"
	}

	return result
}

// calculateOverallStatus 计算整体健康状态
func (a *Aggregator) calculateOverallStatus(checks map[string]CheckResult) Status {
	if len(checks) == 0 {
		return StatusHealthy // 无检查项，默认健康
	}

	hasUnhealthy := false
	hasDegraded := false

	for _, result := range checks {
		switch result.Status {
		case StatusUnhealthy:
			hasUnhealthy = true
		case StatusDegraded:
			hasDegraded = true
		}
	}

	if hasUnhealthy {
		return StatusUnhealthy
	}
	if hasDegraded {
		return StatusDegraded
	}
	return StatusHealthy
}

