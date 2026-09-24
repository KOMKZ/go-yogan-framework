package limiter

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestConcurrencyLimiterEnforcesLimit(t *testing.T) {
	limiter := NewConcurrencyLimiter()
	const limit = 3
	var current, peak int32
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release, err := limiter.Acquire(context.Background(), "provider_a", limit)
			if err != nil {
				t.Errorf("Acquire() error = %v", err)
				return
			}
			defer release()
			now := atomic.AddInt32(&current, 1)
			for {
				old := atomic.LoadInt32(&peak)
				if now <= old || atomic.CompareAndSwapInt32(&peak, old, now) {
					break
				}
			}
			time.Sleep(10 * time.Millisecond)
			atomic.AddInt32(&current, -1)
		}()
	}
	wg.Wait()
	if peak > limit {
		t.Fatalf("peak concurrency = %d, want <= %d", peak, limit)
	}
	if peak == 0 {
		t.Fatalf("peak concurrency should be > 0")
	}
}

func TestConcurrencyLimiterLimitZeroIsUnlimited(t *testing.T) {
	limiter := NewConcurrencyLimiter()
	release, err := limiter.Acquire(context.Background(), "provider_b", 0)
	if err != nil {
		t.Fatalf("Acquire(limit=0) error = %v", err)
	}
	release()
}

func TestConcurrencyLimiterCancelledContext(t *testing.T) {
	limiter := NewConcurrencyLimiter()
	holder, err := limiter.Acquire(context.Background(), "provider_c", 1)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	defer holder()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := limiter.Acquire(ctx, "provider_c", 1); err == nil {
		t.Fatalf("Acquire(cancelled) error = nil, want context error")
	}
}

func TestConcurrencyLimiterFirstLimitWins(t *testing.T) {
	limiter := NewConcurrencyLimiter()
	release, err := limiter.Acquire(context.Background(), "provider_d", 1)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	// 同 key 再次 Acquire 更大 limit 不改变已固定容量，唯一令牌被占用时必须阻塞。
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := limiter.Acquire(ctx, "provider_d", 100); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Acquire() error = %v, want context deadline exceeded", err)
	}

	release()
	next, err := limiter.Acquire(context.Background(), "provider_d", 100)
	if err != nil {
		t.Fatalf("Acquire() after release error = %v", err)
	}
	next()
}
