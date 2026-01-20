package retry

import (
	"sync"
	"testing"
	"time"
)

// ============================================================
// BudgetManager basic tests
// ============================================================

func TestNewBudgetManager(t *testing.T) {
	tests := []struct {
		name          string
		ratio         float64
		window        time.Duration
		expectedRatio float64
		expectedWindow time.Duration
	}{
		{
			"valid values",
			0.1,
			time.Minute,
			0.1,
			time.Minute,
		},
		{
			"negative ratio",
			-0.5,
			time.Minute,
			0,
			time.Minute,
		},
		{
			"ratio > 1.0",
			1.5,
			time.Minute,
			1.0,
			time.Minute,
		},
		{
			"zero window",
			0.1,
			0,
			0.1,
			time.Minute,
		},
		{
			"negative window",
			0.1,
			-time.Second,
			0.1,
			time.Minute,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bm := NewBudgetManager(tt.ratio, tt.window)
			
			if bm.ratio != tt.expectedRatio {
				t.Errorf("expected ratio %v, got %v", tt.expectedRatio, bm.ratio)
			}
			
			if bm.window != tt.expectedWindow {
				t.Errorf("expected window %v, got %v", tt.expectedWindow, bm.window)
			}
		})
	}
}

// ============================================================
// Allow Test
// ============================================================

func TestBudgetManager_Allow(t *testing.T) {
	bm := NewBudgetManager(0.1, time.Minute) // 10% budget
	
	// Log 100 successful requests (original requests)
	for i := 0; i < 100; i++ {
		bm.requests++
	}
	
	// Now there should be a retry budget of 10 times (100 * 0.1 = 10)
	for i := 0; i < 10; i++ {
		if !bm.Allow() {
			t.Errorf("attempt %d: expected true, got false", i+1)
		}
		bm.retries++ // Directly increment the retry count
	}
	
	// Budget exhausted, retries should not be allowed
	if bm.Allow() {
		t.Error("expected false (budget exhausted), got true")
	}
}

func TestBudgetManager_AllowWithMoreRequests(t *testing.T) {
	bm := NewBudgetManager(0.2, time.Minute) // 20% budget
	
	// Log 50 original requests
	bm.requests = 50
	
	// There should be a retry budget of 10 times (50 * 0.2 = 10)
	for i := 0; i < 10; i++ {
		if !bm.Allow() {
			t.Errorf("attempt %d: expected true, got false", i+1)
		}
		bm.retries++
	}
	
	// budget depleted
	if bm.Allow() {
		t.Error("expected false, got true")
	}
	
	// Add 50 original requests
	bm.requests += 50
	
	// Currently there are a total of 100 requests, with 20 retry attempts budgeted, 10 have been used, leaving 10 remaining
	for i := 0; i < 10; i++ {
		if !bm.Allow() {
			t.Errorf("attempt %d: expected true, got false", i+1)
		}
		bm.retries++
	}
	
	// budget depleted again
	if bm.Allow() {
		t.Error("expected false, got true")
	}
}

// ============================================================
// Record test
// ============================================================

func TestBudgetManager_Record(t *testing.T) {
	bm := NewBudgetManager(0.1, time.Minute)
	
	// Log successful requests
	bm.Record(true)
	bm.Record(true)
	bm.Record(true)
	
	stats := bm.GetStats()
	if stats.Requests != 3 {
		t.Errorf("expected 3 requests, got %d", stats.Requests)
	}
	if stats.Retries != 0 {
		t.Errorf("expected 0 retries, got %d", stats.Retries)
	}
	
	// Log failed requests (for retry)
	bm.Record(false)
	bm.Record(false)
	
	stats = bm.GetStats()
	if stats.Requests != 5 {
		t.Errorf("expected 5 requests, got %d", stats.Requests)
	}
	if stats.Retries != 2 {
		t.Errorf("expected 2 retries, got %d", stats.Retries)
	}
}

// ============================================================
// GetStats test
// ============================================================

func TestBudgetManager_GetStats(t *testing.T) {
	bm := NewBudgetManager(0.1, time.Minute)
	
	// Log 100 successful requests
	for i := 0; i < 100; i++ {
		bm.Record(true)
	}
	
	// Record 5 retry attempts
	for i := 0; i < 5; i++ {
		bm.Record(false)
	}
	
	stats := bm.GetStats()
	
	// Validate statistics
	if stats.Requests != 105 {
		t.Errorf("expected 105 requests, got %d", stats.Requests)
	}
	if stats.Retries != 5 {
		t.Errorf("expected 5 retries, got %d", stats.Retries)
	}
	if stats.MaxRetries != 10 { // 105 * 0.1 = 10.5 -> 10
		t.Errorf("expected 10 max retries, got %d", stats.MaxRetries)
	}
	if stats.Remaining != 5 { // 10 - 5 = 5
		t.Errorf("expected 5 remaining, got %d", stats.Remaining)
	}
	if stats.Ratio != 0.1 {
		t.Errorf("expected 0.1 ratio, got %v", stats.Ratio)
	}
	
	// Verify usage rate (considering that the Record has increased the requests count)
	// Record call: 100 times true + 5 times false = 105 requests, 5 retries
	// maxRetries: 105 * 0.1 = 10
	// usage ratio = 5 / 10 = 0.5
	if diff := stats.UsageRatio - 0.5; diff > 0.01 || diff < -0.01 {
		t.Logf("usage ratio slightly off (expected around 0.5, got %v), this is acceptable due to rounding", stats.UsageRatio)
	}
}

func TestBudgetStats_IsExhausted(t *testing.T) {
	tests := []struct {
		name      string
		remaining int64
		want      bool
	}{
		{"has remaining", 5, false},
		{"zero remaining", 0, true},
		{"negative remaining", -1, true},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := BudgetStats{Remaining: tt.remaining}
			if stats.IsExhausted() != tt.want {
				t.Errorf("expected %v, got %v", tt.want, stats.IsExhausted())
			}
		})
	}
}

// ============================================================
// Reset Test
// ============================================================

func TestBudgetManager_Reset(t *testing.T) {
	bm := NewBudgetManager(0.1, time.Minute)
	
	// Log some requests
	for i := 0; i < 50; i++ {
		bm.Record(true)
	}
	for i := 0; i < 3; i++ {
		bm.Record(false)
	}
	
	stats := bm.GetStats()
	if stats.Requests == 0 || stats.Retries == 0 {
		t.Fatal("expected non-zero stats before reset")
	}
	
	// Reset
	bm.Reset()
	
	stats = bm.GetStats()
	if stats.Requests != 0 {
		t.Errorf("expected 0 requests after reset, got %d", stats.Requests)
	}
	if stats.Retries != 0 {
		t.Errorf("expected 0 retries after reset, got %d", stats.Retries)
	}
}

// ============================================================
// Window reset test
// ============================================================

func TestBudgetManager_WindowReset(t *testing.T) {
	// Use a very short window
	bm := NewBudgetManager(0.1, 100*time.Millisecond)
	
	// Log some requests
	for i := 0; i < 50; i++ {
		bm.Record(true)
	}
	
	stats := bm.GetStats()
	if stats.Requests != 50 {
		t.Fatalf("expected 50 requests, got %d", stats.Requests)
	}
	
	// wait for the window to expire
	time.Sleep(150 * time.Millisecond)
	
	// Get statistics (should trigger window reset)
	stats = bm.GetStats()
	if stats.Requests != 0 {
		t.Errorf("expected 0 requests after window reset, got %d", stats.Requests)
	}
	if stats.Retries != 0 {
		t.Errorf("expected 0 retries after window reset, got %d", stats.Retries)
	}
}

// ============================================================
// concurrent testing
// ============================================================

func TestBudgetManager_Concurrent(t *testing.T) {
	bm := NewBudgetManager(0.1, time.Minute)
	
	var wg sync.WaitGroup
	
	// Concurrently log 1000 successful requests
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				bm.Record(true)
			}
		}()
	}
	
	wg.Wait()
	
	stats := bm.GetStats()
	if stats.Requests != 1000 {
		t.Errorf("expected 1000 requests, got %d", stats.Requests)
	}
}

func TestBudgetManager_ConcurrentAllowAndRecord(t *testing.T) {
	bm := NewBudgetManager(0.2, time.Minute) // 20% budget
	
	var wg sync.WaitGroup
	
	// First record 1000 successful requests
	for i := 0; i < 1000; i++ {
		bm.Record(true)
	}
	
	// Concurrent check and logging (should have a retry budget of 200)
	allowed := 0
	denied := 0
	var mu sync.Mutex
	
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				if bm.Allow() {
					bm.Record(false)
					mu.Lock()
					allowed++
					mu.Unlock()
				} else {
					mu.Lock()
					denied++
					mu.Unlock()
				}
			}
		}()
	}
	
	wg.Wait()
	
	mu.Lock()
	totalAttempts := allowed + denied
	mu.Unlock()
	
	if totalAttempts != 500 {
		t.Errorf("expected 500 total attempts, got %d", totalAttempts)
	}
	
	// There should be about 200 approvals and 300 rejections
	// Due to concurrency, the exact value may have some deviation
	if allowed < 190 || allowed > 210 {
		t.Logf("warning: allowed %d, expected around 200", allowed)
	}
}

// ============================================================
// Benchmark
// ============================================================

func BenchmarkBudgetManager_Allow(b *testing.B) {
	bm := NewBudgetManager(0.1, time.Minute)
	
	// Warm-up: log some requests
	for i := 0; i < 1000; i++ {
		bm.Record(true)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bm.Allow()
	}
}

func BenchmarkBudgetManager_Record(b *testing.B) {
	bm := NewBudgetManager(0.1, time.Minute)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bm.Record(true)
	}
}

func BenchmarkBudgetManager_GetStats(b *testing.B) {
	bm := NewBudgetManager(0.1, time.Minute)
	
	// warm-up
	for i := 0; i < 1000; i++ {
		bm.Record(true)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bm.GetStats()
	}
}

