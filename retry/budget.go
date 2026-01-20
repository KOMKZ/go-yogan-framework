package retry

import (
	"sync"
	"time"
)

// BudgetManager retry budget manager
// Used to limit retry traffic to prevent a surge in traffic due to retries amplification
type BudgetManager struct {
	ratio  float64       // budget ratio (e.g., 0.1 = 10%)
	window time.Duration // Statistics window (e.g., 1 minute)
	
	mu      sync.Mutex
	requests int64 // original request count
	retries  int64 // retry request count
	
	windowStart time.Time // window start time
}

// Create Retry Budget Manager
// ratio: budget ratio (0.0 - 1.0), such as 0.1 indicates that the retry request should not exceed 10% of the original request
// window: statistics window, such as time.Minute
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

// Allow check if retry is permitted (budget check)
// return true to indicate allow retry, false to indicate budget depleted
func (b *BudgetManager) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	// Check if window reset is needed
	b.maybeResetWindow()
	
	// Calculate current budget limit
	maxRetries := int64(float64(b.requests) * b.ratio)
	
	// Check if there is remaining budget
	return b.retries < maxRetries
}

// Record the request result (update budget statistics)
// success: Whether the request was successful
func (b *BudgetManager) Record(success bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	// Check if window reset is needed
	b.maybeResetWindow()
	
	// Update statistics
	b.requests++
	if !success {
		b.retries++
	}
}

// GetStats Retrieve budget statistics
func (b *BudgetManager) GetStats() BudgetStats {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	// Check if window reset is needed
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

// Reset budget statistics
func (b *BudgetManager) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	b.requests = 0
	b.retries = 0
	b.windowStart = time.Now()
}

// possiblyResetWindow checks if window reset is needed (internal method, lock before calling)
func (b *BudgetManager) maybeResetWindow() {
	now := time.Now()
	if now.Sub(b.windowStart) >= b.window {
		// Window expired, reset statistics
		b.requests = 0
		b.retries = 0
		b.windowStart = now
	}
}

// calculateUsageRatio Calculate budget usage ratio (internal method, lock must be acquired before calling)
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

// BudgetStatistics
type BudgetStats struct {
	Requests    int64         // original request count
	Retries     int64         // retry request count
	MaxRetries  int64         // Maximum allowed retry count
	Remaining   int64         // remaining budget
	Ratio       float64       // budget ratio
	UsageRatio  float64       // Budget usage rate (0.0 - 1.0)
	WindowStart time.Time     // window start time
	WindowEnd   time.Time     // window end time
}

// determines if the budget is depleted
func (s *BudgetStats) IsExhausted() bool {
	return s.Remaining <= 0
}

// UsagePercent returns the budget usage percentage (0 - 100)
func (s *BudgetStats) UsagePercent() float64 {
	return s.UsageRatio * 100
}

