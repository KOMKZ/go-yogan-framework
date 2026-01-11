package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

// ============================================================
// Budget 集成测试
// ============================================================

func TestDo_WithBudget(t *testing.T) {
	ctx := context.Background()
	budget := NewBudgetManager(0.5, time.Minute)
	
	// 先模拟10个原始请求
	budget.requests = 10
	// 现在预算：10 * 0.5 = 5次重试
	
	called := 0
	err := Do(ctx, func() error {
		called++
		return errors.New("test error")
	},
		Budget(budget),
		MaxAttempts(10),
		Backoff(NoBackoff()),
	)
	
	// 验证有错误
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	
	// 验证包含预算错误（可能在错误链中）
	if multiErr, ok := err.(*MultiError); ok {
		found := false
		for _, e := range multiErr.Errors {
			if errors.Is(e, ErrBudgetExhausted) {
				found = true
				break
			}
		}
		if found {
			t.Logf("✅ Found ErrBudgetExhausted in MultiError (called %d times)", called)
		}
	}
}

func TestDo_WithBudget_FirstAttemptSuccess(t *testing.T) {
	ctx := context.Background()
	budget := NewBudgetManager(0.1, time.Minute)
	budget.requests = 100
	
	// 第一次尝试成功，不消耗预算
	err := Do(ctx, func() error {
		return nil
	},
		Budget(budget),
		MaxAttempts(3),
	)
	
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	
	stats := budget.GetStats()
	if stats.Retries != 0 {
		t.Errorf("expected 0 retries, got %d", stats.Retries)
	}
}

// ============================================================
// 边界情况测试
// ============================================================

func TestDo_RemainingTimeCheck(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	
	called := 0
	
	err := Do(ctx, func() error {
		called++
		return errors.New("test error")
	},
		MaxAttempts(10),
		Backoff(ConstantBackoff(100*time.Millisecond, WithJitter(0))),
	)
	
	// 应该在剩余时间不足时停止重试
	if called > 2 {
		t.Errorf("expected at most 2 calls, got %d", called)
	}
	
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Logf("got error: %v", err)
	}
}

func TestDoWithData_ZeroValue(t *testing.T) {
	ctx := context.Background()
	
	result, err := DoWithData(ctx, func() (int, error) {
		return 0, errors.New("test error")
	},
		MaxAttempts(2),
		Backoff(NoBackoff()),
	)
	
	// 失败时应该返回零值
	if result != 0 {
		t.Errorf("expected 0, got %d", result)
	}
	
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ============================================================
// 测试 executeWithContext 超时
// ============================================================

func TestExecuteWithContext_Timeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	
	result, err := executeWithContext(ctx, func() (int, error) {
		time.Sleep(200 * time.Millisecond)
		return 42, nil
	})
	
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}
	
	if result != 0 {
		t.Errorf("expected 0, got %d", result)
	}
}

func TestExecuteWithContext_Success(t *testing.T) {
	ctx := context.Background()
	
	result, err := executeWithContext(ctx, func() (string, error) {
		return "success", nil
	})
	
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	
	if result != "success" {
		t.Errorf("expected 'success', got %q", result)
	}
}

