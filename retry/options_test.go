package retry

import (
	"errors"
	"testing"
	"time"
)

// ============================================================
// MultiError test
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
// Options test
// ============================================================

func TestOptions_Nil(t *testing.T) {
	cfg := defaultConfig()
	
	// Test that nil option does not cause panic
	MaxAttempts(0)(cfg)          // Invalid value, should be ignored
	Backoff(nil)(cfg)             // nil, should be ignored
	Condition(nil)(cfg)           // nil, should be ignored
	OnRetry(nil)(cfg)             // nil, allow
	Timeout(0)(cfg)               // 0 should be ignored
	Timeout(-time.Second)(cfg)    // Negative numbers should be ignored
	Budget(nil)(cfg)              // nil, allow
	
	// Verify that default values have not been corrupted
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
	
	// Test valid options
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

