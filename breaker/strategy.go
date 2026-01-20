package breaker

// Fusion strategy interface
type Strategy interface {
	// ShouldOpen determines whether circuit breaking should be applied
	ShouldOpen(snapshot *MetricsSnapshot, config ResourceConfig) bool
	
	// Name Strategy Name
	Name() string
}

// error rate strategy
type errorRateStrategy struct{}

func (s *errorRateStrategy) Name() string {
	return "error_rate"
}

func (s *errorRateStrategy) ShouldOpen(snapshot *MetricsSnapshot, config ResourceConfig) bool {
	// Minimum request count check
	if snapshot.TotalRequests < int64(config.MinRequests) {
		return false
	}
	
	// Error rate check
	return snapshot.ErrorRate >= config.ErrorRateThreshold
}

// slowCallRateStrategy slow call rate strategy
type slowCallRateStrategy struct{}

func (s *slowCallRateStrategy) Name() string {
	return "slow_call_rate"
}

func (s *slowCallRateStrategy) ShouldOpen(snapshot *MetricsSnapshot, config ResourceConfig) bool {
	// Minimum request count check
	if snapshot.TotalRequests < int64(config.MinRequests) {
		return false
	}
	
	// Slow call rate check
	return snapshot.SlowCallRate >= config.SlowRateThreshold
}

// consecutiveFailuresStrategy consecutive failure strategy
type consecutiveFailuresStrategy struct {
	failureCount int
}

func (s *consecutiveFailuresStrategy) Name() string {
	return "consecutive_failures"
}

func (s *consecutiveFailuresStrategy) ShouldOpen(snapshot *MetricsSnapshot, config ResourceConfig) bool {
	// Consecutive failure count check
	return s.failureCount >= config.ConsecutiveFailures
}

// RecordSuccess (reset count)
func (s *consecutiveFailuresStrategy) RecordSuccess() {
	s.failureCount = 0
}

// RecordFailure record failure (increase count)
func (s *consecutiveFailuresStrategy) RecordFailure() {
	s.failureCount++
}

// Get strategy by name
func GetStrategyByName(name string) Strategy {
	switch name {
	case "error_rate":
		return &errorRateStrategy{}
	case "slow_call_rate":
		return &slowCallRateStrategy{}
	case "consecutive_failures":
		return &consecutiveFailuresStrategy{}
	default:
		return &errorRateStrategy{} // Use error rate strategy by default
	}
}

