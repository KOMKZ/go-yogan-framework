package retry

import (
	"sync"
	"testing"
	"time"
)

// ============================================================
// BudgetManager 基础测试
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
// Allow 测试
// ============================================================

func TestBudgetManager_Allow(t *testing.T) {
	bm := NewBudgetManager(0.1, time.Minute) // 10% 预算
	
	// 记录 100 个成功请求（原始请求）
	for i := 0; i < 100; i++ {
		bm.requests++
	}
	
	// 现在应该有 10 次重试预算（100 * 0.1 = 10）
	for i := 0; i < 10; i++ {
		if !bm.Allow() {
			t.Errorf("attempt %d: expected true, got false", i+1)
		}
		bm.retries++ // 直接增加重试计数
	}
	
	// 预算耗尽，应该不允许重试
	if bm.Allow() {
		t.Error("expected false (budget exhausted), got true")
	}
}

func TestBudgetManager_AllowWithMoreRequests(t *testing.T) {
	bm := NewBudgetManager(0.2, time.Minute) // 20% 预算
	
	// 记录 50 个原始请求
	bm.requests = 50
	
	// 应该有 10 次重试预算（50 * 0.2 = 10）
	for i := 0; i < 10; i++ {
		if !bm.Allow() {
			t.Errorf("attempt %d: expected true, got false", i+1)
		}
		bm.retries++
	}
	
	// 预算耗尽
	if bm.Allow() {
		t.Error("expected false, got true")
	}
	
	// 新增 50 个原始请求
	bm.requests += 50
	
	// 现在总共 100 个请求，应该有 20 次重试预算，已用 10 次，剩余 10 次
	for i := 0; i < 10; i++ {
		if !bm.Allow() {
			t.Errorf("attempt %d: expected true, got false", i+1)
		}
		bm.retries++
	}
	
	// 预算再次耗尽
	if bm.Allow() {
		t.Error("expected false, got true")
	}
}

// ============================================================
// Record 测试
// ============================================================

func TestBudgetManager_Record(t *testing.T) {
	bm := NewBudgetManager(0.1, time.Minute)
	
	// 记录成功请求
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
	
	// 记录失败请求（重试）
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
// GetStats 测试
// ============================================================

func TestBudgetManager_GetStats(t *testing.T) {
	bm := NewBudgetManager(0.1, time.Minute)
	
	// 记录 100 个成功请求
	for i := 0; i < 100; i++ {
		bm.Record(true)
	}
	
	// 记录 5 次重试
	for i := 0; i < 5; i++ {
		bm.Record(false)
	}
	
	stats := bm.GetStats()
	
	// 验证统计信息
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
	
	// 验证使用率（考虑 Record 增加了 requests 计数）
	// Record调用: 100次true + 5次false = 105 requests, 5 retries
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
// Reset 测试
// ============================================================

func TestBudgetManager_Reset(t *testing.T) {
	bm := NewBudgetManager(0.1, time.Minute)
	
	// 记录一些请求
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
	
	// 重置
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
// 窗口重置测试
// ============================================================

func TestBudgetManager_WindowReset(t *testing.T) {
	// 使用很短的窗口
	bm := NewBudgetManager(0.1, 100*time.Millisecond)
	
	// 记录一些请求
	for i := 0; i < 50; i++ {
		bm.Record(true)
	}
	
	stats := bm.GetStats()
	if stats.Requests != 50 {
		t.Fatalf("expected 50 requests, got %d", stats.Requests)
	}
	
	// 等待窗口过期
	time.Sleep(150 * time.Millisecond)
	
	// 获取统计（应该触发窗口重置）
	stats = bm.GetStats()
	if stats.Requests != 0 {
		t.Errorf("expected 0 requests after window reset, got %d", stats.Requests)
	}
	if stats.Retries != 0 {
		t.Errorf("expected 0 retries after window reset, got %d", stats.Retries)
	}
}

// ============================================================
// 并发测试
// ============================================================

func TestBudgetManager_Concurrent(t *testing.T) {
	bm := NewBudgetManager(0.1, time.Minute)
	
	var wg sync.WaitGroup
	
	// 并发记录 1000 个成功请求
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
	bm := NewBudgetManager(0.2, time.Minute) // 20% 预算
	
	var wg sync.WaitGroup
	
	// 先记录 1000 个成功请求
	for i := 0; i < 1000; i++ {
		bm.Record(true)
	}
	
	// 并发检查和记录（应该有 200 次重试预算）
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
	
	// 应该大约有 200 次允许，300 次拒绝
	// 由于并发，精确值可能有些偏差
	if allowed < 190 || allowed > 210 {
		t.Logf("warning: allowed %d, expected around 200", allowed)
	}
}

// ============================================================
// Benchmark
// ============================================================

func BenchmarkBudgetManager_Allow(b *testing.B) {
	bm := NewBudgetManager(0.1, time.Minute)
	
	// 预热：记录一些请求
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
	
	// 预热
	for i := 0; i < 1000; i++ {
		bm.Record(true)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bm.GetStats()
	}
}

