package retry

import (
	"errors"
	"testing"
	"time"
)

// ============================================================
// MultiError 测试
// ============================================================

func TestMultiError_Empty(t *testing.T) {
	multiErr := &MultiError{
		Errors:   []error{},
		Attempts: 0,
	}
	
	if multiErr.Error() != "retry failed: no errors" {
		t.Errorf("expected 'retry failed: no errors', got %q", multiErr.Error())
	}
	
	if multiErr.Unwrap() != nil {
		t.Errorf("expected nil, got %v", multiErr.Unwrap())
	}
	
	if multiErr.FirstError() != nil {
		t.Errorf("expected nil, got %v", multiErr.FirstError())
	}
	
	if multiErr.LastError() != nil {
		t.Errorf("expected nil, got %v", multiErr.LastError())
	}
	
	if multiErr.AllErrors() != "" {
		t.Errorf("expected empty string, got %q", multiErr.AllErrors())
	}
}

// ============================================================
// Options 测试
// ============================================================

func TestOptions_Nil(t *testing.T) {
	cfg := defaultConfig()
	
	// 测试 nil 选项不会导致 panic
	MaxAttempts(0)(cfg)          // 无效值，应该被忽略
	Backoff(nil)(cfg)             // nil，应该被忽略
	Condition(nil)(cfg)           // nil，应该被忽略
	OnRetry(nil)(cfg)             // nil，允许
	Timeout(0)(cfg)               // 0，应该被忽略
	Timeout(-time.Second)(cfg)    // 负数，应该被忽略
	Budget(nil)(cfg)              // nil，允许
	
	// 验证默认值没有被破坏
	if cfg.maxAttempts != 3 {
		t.Errorf("expected 3, got %d", cfg.maxAttempts)
	}
	
	if cfg.backoff == nil {
		t.Error("backoff should not be nil")
	}
	
	if cfg.condition == nil {
		t.Error("condition should not be nil")
	}
}

func TestOptions_Valid(t *testing.T) {
	cfg := defaultConfig()
	
	// 测试有效选项
	MaxAttempts(5)(cfg)
	if cfg.maxAttempts != 5 {
		t.Errorf("expected 5, got %d", cfg.maxAttempts)
	}
	
	customBackoff := NoBackoff()
	Backoff(customBackoff)(cfg)
	if cfg.backoff != customBackoff {
		t.Error("backoff not set correctly")
	}
	
	customCondition := NeverRetry()
	Condition(customCondition)(cfg)
	if cfg.condition != customCondition {
		t.Error("condition not set correctly")
	}
	
	called := false
	OnRetry(func(attempt int, err error) {
		called = true
	})(cfg)
	cfg.onRetry(1, errors.New("test"))
	if !called {
		t.Error("onRetry not set correctly")
	}
	
	Timeout(time.Second)(cfg)
	if cfg.timeout != time.Second {
		t.Errorf("expected 1s, got %v", cfg.timeout)
	}
	
	budget := NewBudgetManager(0.1, time.Minute)
	Budget(budget)(cfg)
	if cfg.budget != budget {
		t.Error("budget not set correctly")
	}
}

