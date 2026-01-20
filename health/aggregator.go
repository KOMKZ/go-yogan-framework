package health

import (
	"context"
	"sync"
	"time"
)

// HealthCheck Aggregator
// Unified management of multiple health check items
type Aggregator struct {
	checkers []Checker
	timeout  time.Duration
	mu       sync.RWMutex
	metadata map[string]interface{}
}

// Create health check aggregator
func NewAggregator(timeout time.Duration) *Aggregator {
	if timeout <= 0 {
		timeout = 5 * time.Second // Default 5 second timeout
	}
	return &Aggregator{
		checkers: make([]Checker, 0),
		timeout:  timeout,
		metadata: make(map[string]interface{}),
	}
}

// Register health check item
func (a *Aggregator) Register(checker Checker) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.checkers = append(a.checkers, checker)
}

// SetMetadata Set metadata
func (a *Aggregator) SetMetadata(key string, value interface{}) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.metadata[key] = value
}

// Check all health checks execution
func (a *Aggregator) Check(ctx context.Context) *Response {
	start := time.Now()

	// Create timeout context
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

	// Concurrently execute all checks
	results := make(chan CheckResult, len(checkers))
	for _, checker := range checkers {
		go func(c Checker) {
			results <- a.checkOne(checkCtx, c)
		}(checker)
	}

	// Collect results
	checks := make(map[string]CheckResult)
	for i := 0; i < len(checkers); i++ {
		result := <-results
		checks[result.Name] = result
	}

	// Calculate overall status
	overallStatus := a.calculateOverallStatus(checks)

	return &Response{
		Status:    overallStatus,
		Timestamp: time.Now(),
		Duration:  time.Since(start),
		Checks:    checks,
		Metadata:  metadata,
	}
}

// execute single check
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

// calculateOverallStatus Calculate overall health status
func (a *Aggregator) calculateOverallStatus(checks map[string]CheckResult) Status {
	if len(checks) == 0 {
		return StatusHealthy // No checks performed, default healthy
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

