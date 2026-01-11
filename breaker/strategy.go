package breaker

// Strategy 熔断策略接口
type Strategy interface {
	// ShouldOpen 判断是否应该熔断
	ShouldOpen(snapshot *MetricsSnapshot, config ResourceConfig) bool
	
	// Name 策略名称
	Name() string
}

// errorRateStrategy 错误率策略
type errorRateStrategy struct{}

func (s *errorRateStrategy) Name() string {
	return "error_rate"
}

func (s *errorRateStrategy) ShouldOpen(snapshot *MetricsSnapshot, config ResourceConfig) bool {
	// 最小请求数检查
	if snapshot.TotalRequests < int64(config.MinRequests) {
		return false
	}
	
	// 错误率检查
	return snapshot.ErrorRate >= config.ErrorRateThreshold
}

// slowCallRateStrategy 慢调用率策略
type slowCallRateStrategy struct{}

func (s *slowCallRateStrategy) Name() string {
	return "slow_call_rate"
}

func (s *slowCallRateStrategy) ShouldOpen(snapshot *MetricsSnapshot, config ResourceConfig) bool {
	// 最小请求数检查
	if snapshot.TotalRequests < int64(config.MinRequests) {
		return false
	}
	
	// 慢调用率检查
	return snapshot.SlowCallRate >= config.SlowRateThreshold
}

// consecutiveFailuresStrategy 连续失败策略
type consecutiveFailuresStrategy struct {
	failureCount int
}

func (s *consecutiveFailuresStrategy) Name() string {
	return "consecutive_failures"
}

func (s *consecutiveFailuresStrategy) ShouldOpen(snapshot *MetricsSnapshot, config ResourceConfig) bool {
	// 连续失败次数检查
	return s.failureCount >= config.ConsecutiveFailures
}

// RecordSuccess 记录成功（重置计数）
func (s *consecutiveFailuresStrategy) RecordSuccess() {
	s.failureCount = 0
}

// RecordFailure 记录失败（增加计数）
func (s *consecutiveFailuresStrategy) RecordFailure() {
	s.failureCount++
}

// GetStrategyByName 根据名称获取策略
func GetStrategyByName(name string) Strategy {
	switch name {
	case "error_rate":
		return &errorRateStrategy{}
	case "slow_call_rate":
		return &slowCallRateStrategy{}
	case "consecutive_failures":
		return &consecutiveFailuresStrategy{}
	default:
		return &errorRateStrategy{} // 默认使用错误率策略
	}
}

