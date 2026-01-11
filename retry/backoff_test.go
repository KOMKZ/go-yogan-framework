package retry

import (
	"testing"
	"time"
)

// ============================================================
// 指数退避测试
// ============================================================

func TestExponentialBackoff_Basic(t *testing.T) {
	backoff := ExponentialBackoff(time.Second, WithJitter(0)) // 禁用抖动，方便测试
	
	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{1, 1 * time.Second},  // 1s * 2^0 = 1s
		{2, 2 * time.Second},  // 1s * 2^1 = 2s
		{3, 4 * time.Second},  // 1s * 2^2 = 4s
		{4, 8 * time.Second},  // 1s * 2^3 = 8s
		{5, 16 * time.Second}, // 1s * 2^4 = 16s
	}
	
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := backoff.Next(tt.attempt)
			if got != tt.expected {
				t.Errorf("attempt %d: got %v, want %v", tt.attempt, got, tt.expected)
			}
		})
	}
}

func TestExponentialBackoff_WithMultiplier(t *testing.T) {
	backoff := ExponentialBackoff(
		time.Second,
		WithMultiplier(3.0), // 3 倍增长
		WithJitter(0),
	)
	
	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{1, 1 * time.Second},  // 1s * 3^0 = 1s
		{2, 3 * time.Second},  // 1s * 3^1 = 3s
		{3, 9 * time.Second},  // 1s * 3^2 = 9s
		{4, 27 * time.Second}, // 1s * 3^3 = 27s
	}
	
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := backoff.Next(tt.attempt)
			if got != tt.expected {
				t.Errorf("attempt %d: got %v, want %v", tt.attempt, got, tt.expected)
			}
		})
	}
}

func TestExponentialBackoff_WithMaxDelay(t *testing.T) {
	backoff := ExponentialBackoff(
		time.Second,
		WithMaxDelay(5*time.Second), // 最大 5s
		WithJitter(0),
	)
	
	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{1, 1 * time.Second}, // 1s
		{2, 2 * time.Second}, // 2s
		{3, 4 * time.Second}, // 4s
		{4, 5 * time.Second}, // 8s -> 限制为 5s
		{5, 5 * time.Second}, // 16s -> 限制为 5s
	}
	
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := backoff.Next(tt.attempt)
			if got != tt.expected {
				t.Errorf("attempt %d: got %v, want %v", tt.attempt, got, tt.expected)
			}
		})
	}
}

func TestExponentialBackoff_WithJitter(t *testing.T) {
	backoff := ExponentialBackoff(
		time.Second,
		WithJitter(0.2), // 20% 抖动
	)
	
	// 测试抖动范围
	base := time.Second
	minDelay := time.Duration(float64(base) * 0.8) // 0.8s
	maxDelay := time.Duration(float64(base) * 1.2) // 1.2s
	
	// 多次测试，验证抖动范围
	for i := 0; i < 100; i++ {
		delay := backoff.Next(1)
		if delay < minDelay || delay > maxDelay {
			t.Errorf("delay %v out of range [%v, %v]", delay, minDelay, maxDelay)
		}
	}
}

func TestExponentialBackoff_ZeroAttempt(t *testing.T) {
	backoff := ExponentialBackoff(time.Second)
	
	got := backoff.Next(0)
	if got != 0 {
		t.Errorf("got %v, want 0", got)
	}
}

func TestExponentialBackoff_NegativeAttempt(t *testing.T) {
	backoff := ExponentialBackoff(time.Second)
	
	got := backoff.Next(-1)
	if got != 0 {
		t.Errorf("got %v, want 0", got)
	}
}

// ============================================================
// 线性退避测试
// ============================================================

func TestLinearBackoff_Basic(t *testing.T) {
	backoff := LinearBackoff(time.Second, WithJitter(0))
	
	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{1, 1 * time.Second},
		{2, 2 * time.Second},
		{3, 3 * time.Second},
		{4, 4 * time.Second},
		{5, 5 * time.Second},
	}
	
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := backoff.Next(tt.attempt)
			if got != tt.expected {
				t.Errorf("attempt %d: got %v, want %v", tt.attempt, got, tt.expected)
			}
		})
	}
}

func TestLinearBackoff_WithMaxDelay(t *testing.T) {
	backoff := LinearBackoff(
		time.Second,
		WithMaxDelay(3*time.Second),
		WithJitter(0),
	)
	
	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{1, 1 * time.Second},
		{2, 2 * time.Second},
		{3, 3 * time.Second},
		{4, 3 * time.Second}, // 限制为 3s
		{5, 3 * time.Second}, // 限制为 3s
	}
	
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := backoff.Next(tt.attempt)
			if got != tt.expected {
				t.Errorf("attempt %d: got %v, want %v", tt.attempt, got, tt.expected)
			}
		})
	}
}

func TestLinearBackoff_ZeroAttempt(t *testing.T) {
	backoff := LinearBackoff(time.Second)
	
	got := backoff.Next(0)
	if got != 0 {
		t.Errorf("got %v, want 0", got)
	}
}

// ============================================================
// 固定退避测试
// ============================================================

func TestConstantBackoff_Basic(t *testing.T) {
	backoff := ConstantBackoff(2*time.Second, WithJitter(0))
	
	// 所有尝试都应该返回相同的延迟
	for attempt := 1; attempt <= 5; attempt++ {
		got := backoff.Next(attempt)
		expected := 2 * time.Second
		if got != expected {
			t.Errorf("attempt %d: got %v, want %v", attempt, got, expected)
		}
	}
}

func TestConstantBackoff_WithJitter(t *testing.T) {
	backoff := ConstantBackoff(
		time.Second,
		WithJitter(0.2),
	)
	
	minDelay := time.Duration(float64(time.Second) * 0.8)
	maxDelay := time.Duration(float64(time.Second) * 1.2)
	
	for i := 0; i < 100; i++ {
		delay := backoff.Next(1)
		if delay < minDelay || delay > maxDelay {
			t.Errorf("delay %v out of range [%v, %v]", delay, minDelay, maxDelay)
		}
	}
}

func TestConstantBackoff_ZeroAttempt(t *testing.T) {
	backoff := ConstantBackoff(time.Second)
	
	got := backoff.Next(0)
	if got != 0 {
		t.Errorf("got %v, want 0", got)
	}
}

// ============================================================
// 无退避测试
// ============================================================

func TestNoBackoff(t *testing.T) {
	backoff := NoBackoff()
	
	for attempt := 1; attempt <= 5; attempt++ {
		got := backoff.Next(attempt)
		if got != 0 {
			t.Errorf("attempt %d: got %v, want 0", attempt, got)
		}
	}
}

// ============================================================
// 抖动测试
// ============================================================

func TestApplyJitter(t *testing.T) {
	tests := []struct {
		name   string
		delay  float64
		jitter float64
		min    float64
		max    float64
	}{
		{
			name:   "20% jitter",
			delay:  1000,
			jitter: 0.2,
			min:    800,  // 1000 * 0.8
			max:    1200, // 1000 * 1.2
		},
		{
			name:   "50% jitter",
			delay:  1000,
			jitter: 0.5,
			min:    500,  // 1000 * 0.5
			max:    1500, // 1000 * 1.5
		},
		{
			name:   "100% jitter",
			delay:  1000,
			jitter: 1.0,
			min:    0,    // 1000 * 0
			max:    2000, // 1000 * 2
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 多次测试，验证随机范围
			for i := 0; i < 100; i++ {
				result := applyJitter(tt.delay, tt.jitter)
				if result < tt.min || result > tt.max {
					t.Errorf("result %v out of range [%v, %v]", result, tt.min, tt.max)
				}
			}
		})
	}
}

func TestApplyJitter_ZeroJitter(t *testing.T) {
	delay := 1000.0
	result := applyJitter(delay, 0)
	if result != delay {
		t.Errorf("got %v, want %v", result, delay)
	}
}

func TestApplyJitter_NegativeResult(t *testing.T) {
	// 极端情况：小延迟 + 大抖动可能导致负数
	delay := 10.0
	jitter := 1.0
	
	for i := 0; i < 100; i++ {
		result := applyJitter(delay, jitter)
		if result < 0 {
			t.Errorf("result %v should not be negative", result)
		}
	}
}

// ============================================================
// 选项测试
// ============================================================

func TestBackoffOptions_InvalidValues(t *testing.T) {
	// 测试无效的选项值不会导致 panic
	backoff := ExponentialBackoff(
		time.Second,
		WithMultiplier(-1.0),     // 无效，应该被忽略
		WithMaxDelay(-1),          // 无效，应该被忽略
		WithJitter(-0.1),          // 无效，应该被忽略
		WithJitter(1.5),           // 无效，应该被忽略
	)
	
	// 应该使用默认值
	delay := backoff.Next(1)
	if delay <= 0 {
		t.Errorf("delay should be positive, got %v", delay)
	}
}

// ============================================================
// Benchmark
// ============================================================

func BenchmarkExponentialBackoff(b *testing.B) {
	backoff := ExponentialBackoff(time.Second)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backoff.Next(i % 10)
	}
}

func BenchmarkLinearBackoff(b *testing.B) {
	backoff := LinearBackoff(time.Second)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backoff.Next(i % 10)
	}
}

func BenchmarkConstantBackoff(b *testing.B) {
	backoff := ConstantBackoff(time.Second)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backoff.Next(i % 10)
	}
}

