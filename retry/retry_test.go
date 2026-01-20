package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

// ============================================================
// Perform basic tests
// ============================================================

func TestDo_Success(t *testing.T) {
	ctx := context.Background()
	called := 0
	
	err := Do(ctx, func() error {
		called++
		return nil
	})
	
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	
	if called != 1 {
		t.Errorf("expected 1 call, got %d", called)
	}
}

func TestDo_FailThenSuccess(t *testing.T) {
	ctx := context.Background()
	called := 0
	
	err := Do(ctx, func() error {
		called++
		if called < 3 {
			return errors.New("temporary error")
		}
		return nil
	}, MaxAttempts(5))
	
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	
	if called != 3 {
		t.Errorf("expected 3 calls, got %d", called)
	}
}

func TestDo_AllFailed(t *testing.T) {
	ctx := context.Background()
	called := 0
	testErr := errors.New("test error")
	
	err := Do(ctx, func() error {
		called++
		return testErr
	}, MaxAttempts(3), Backoff(NoBackoff()))
	
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	
	if called != 3 {
		t.Errorf("expected 3 calls, got %d", called)
	}
	
	// Validate MultiError
	var multiErr *MultiError
	if !errors.As(err, &multiErr) {
		t.Fatalf("expected MultiError, got %T", err)
	}
	
	if multiErr.Attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", multiErr.Attempts)
	}
	
	if len(multiErr.Errors) != 3 {
		t.Errorf("expected 3 errors, got %d", len(multiErr.Errors))
	}
}

// ============================================================
// Test With Data
// ============================================================

func TestDoWithData_Success(t *testing.T) {
	ctx := context.Background()
	called := 0
	
	result, err := DoWithData(ctx, func() (string, error) {
		called++
		return "success", nil
	})
	
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	
	if result != "success" {
		t.Errorf("expected 'success', got %q", result)
	}
	
	if called != 1 {
		t.Errorf("expected 1 call, got %d", called)
	}
}

func TestDoWithData_FailThenSuccess(t *testing.T) {
	ctx := context.Background()
	called := 0
	
	result, err := DoWithData(ctx, func() (int, error) {
		called++
		if called < 2 {
			return 0, errors.New("temporary error")
		}
		return 42, nil
	}, MaxAttempts(3), Backoff(NoBackoff()))
	
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	
	if result != 42 {
		t.Errorf("expected 42, got %d", result)
	}
	
	if called != 2 {
		t.Errorf("expected 2 calls, got %d", called)
	}
}

// ============================================================
// Context cancel test
// ============================================================

func TestDo_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	called := 0
	
	// Cancel Context on the second call
	err := Do(ctx, func() error {
		called++
		if called == 2 {
			cancel()
			time.Sleep(10 * time.Millisecond) // wait for cancellation to take effect
		}
		return errors.New("test error")
	}, MaxAttempts(5), Backoff(ConstantBackoff(50*time.Millisecond)))
	
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	
	// Should stop retrying after cancellation
	if called > 3 {
		t.Errorf("expected at most 3 calls, got %d", called)
	}
}

func TestDo_ContextDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	
	called := 0
	
	err := Do(ctx, func() error {
		called++
		time.Sleep(30 * time.Millisecond)
		return errors.New("test error")
	}, MaxAttempts(10), Backoff(NoBackoff()))
	
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}
	
	// Should stop retrying after timeout
	if called > 4 {
		t.Errorf("expected at most 4 calls, got %d (timeout should stop retry)", called)
	}
}

// ============================================================
// backoff strategy test
// ============================================================

func TestDo_WithBackoff(t *testing.T) {
	ctx := context.Background()
	called := 0
	start := time.Now()
	
	err := Do(ctx, func() error {
		called++
		return errors.New("test error")
	},
		MaxAttempts(3),
		Backoff(ConstantBackoff(100*time.Millisecond, WithJitter(0))),
	)
	
	elapsed := time.Since(start)
	
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	
	if called != 3 {
		t.Errorf("expected 3 calls, got %d", called)
	}
	
	// Validate backoff time: 2 attempts backoff * 100ms = 200ms
	expectedMin := 200 * time.Millisecond
	expectedMax := 300 * time.Millisecond
	if elapsed < expectedMin || elapsed > expectedMax {
		t.Errorf("expected elapsed time in [%v, %v], got %v", expectedMin, expectedMax, elapsed)
	}
}

// ============================================================
// Retry condition test
// ============================================================

var ErrRetryable = errors.New("retryable error")
var ErrNotRetryable = errors.New("not retryable error")

func TestDo_WithCondition(t *testing.T) {
	ctx := context.Background()
	called := 0
	
	err := Do(ctx, func() error {
		called++
		if called == 1 {
			return ErrRetryable
		}
		return ErrNotRetryable
	},
		MaxAttempts(5),
		Condition(RetryOnError(ErrRetryable)),
		Backoff(NoBackoff()),
	)
	
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	
	// First return ErrRetryable, should retry
	// The second return of ErrNotRetryable, should not retry
	if called != 2 {
		t.Errorf("expected 2 calls, got %d", called)
	}
	
	// The last error should be ErrNotRetryable
	if !errors.Is(err, ErrNotRetryable) {
		t.Errorf("expected ErrNotRetryable, got %v", err)
	}
}

// ============================================================
// Retry callback test
// ============================================================

func TestDo_OnRetry(t *testing.T) {
	ctx := context.Background()
	called := 0
	retryAttempts := []int{}
	retryErrors := []error{}
	
	err := Do(ctx, func() error {
		called++
		return errors.New("test error")
	},
		MaxAttempts(3),
		OnRetry(func(attempt int, err error) {
			retryAttempts = append(retryAttempts, attempt)
			retryErrors = append(retryErrors, err)
		}),
		Backoff(NoBackoff()),
	)
	
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	
	if called != 3 {
		t.Errorf("expected 3 calls, got %d", called)
	}
	
	// OnRetry should be called twice (after the first failure and after the second failure)
	if len(retryAttempts) != 2 {
		t.Errorf("expected 2 retry callbacks, got %d", len(retryAttempts))
	}
	
	expectedAttempts := []int{1, 2}
	for i, attempt := range retryAttempts {
		if attempt != expectedAttempts[i] {
			t.Errorf("retry callback %d: expected attempt %d, got %d", i, expectedAttempts[i], attempt)
		}
	}
}

// ============================================================
// Single timeout test
// ============================================================

func TestDo_WithTimeout(t *testing.T) {
	ctx := context.Background()
	called := 0
	
	err := Do(ctx, func() error {
		called++
		time.Sleep(200 * time.Millisecond)
		return nil
	},
		MaxAttempts(3),
		Timeout(100*time.Millisecond),
		Backoff(NoBackoff()),
	)
	
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}
	
	// Times out every time, should retry 3 times
	if called != 3 {
		t.Errorf("expected 3 calls, got %d", called)
	}
}

// ============================================================
// MultiError test
// ============================================================

func TestMultiError_Error(t *testing.T) {
	err1 := errors.New("error 1")
	err2 := errors.New("error 2")
	err3 := errors.New("error 3")
	
	multiErr := &MultiError{
		Errors:   []error{err1, err2, err3},
		Attempts: 3,
	}
	
	// Error() should return the last error
	if multiErr.Error() != "error 3" {
		t.Errorf("expected 'error 3', got %q", multiErr.Error())
	}
}

func TestMultiError_Unwrap(t *testing.T) {
	err1 := errors.New("error 1")
	err2 := errors.New("error 2")
	err3 := errors.New("error 3")
	
	multiErr := &MultiError{
		Errors:   []error{err1, err2, err3},
		Attempts: 3,
	}
	
	// Unwrap() should return the last error
	if multiErr.Unwrap() != err3 {
		t.Errorf("expected err3, got %v", multiErr.Unwrap())
	}
}

func TestMultiError_AllErrors(t *testing.T) {
	err1 := errors.New("error 1")
	err2 := errors.New("error 2")
	
	multiErr := &MultiError{
		Errors:   []error{err1, err2},
		Attempts: 2,
	}
	
	allErrors := multiErr.AllErrors()
	expected := "retry failed after 2 attempts:\n  attempt 1: error 1\n  attempt 2: error 2"
	
	if allErrors != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, allErrors)
	}
}

func TestMultiError_FirstAndLast(t *testing.T) {
	err1 := errors.New("error 1")
	err2 := errors.New("error 2")
	err3 := errors.New("error 3")
	
	multiErr := &MultiError{
		Errors:   []error{err1, err2, err3},
		Attempts: 3,
	}
	
	if multiErr.FirstError() != err1 {
		t.Errorf("expected err1, got %v", multiErr.FirstError())
	}
	
	if multiErr.LastError() != err3 {
		t.Errorf("expected err3, got %v", multiErr.LastError())
	}
}

// ============================================================
// helper function test
// ============================================================

func TestIsMaxAttemptsExceeded(t *testing.T) {
	multiErr := &MultiError{
		Errors:   []error{errors.New("error")},
		Attempts: 3,
	}
	
	if !IsMaxAttemptsExceeded(multiErr) {
		t.Error("expected true, got false")
	}
	
	normalErr := errors.New("normal error")
	if IsMaxAttemptsExceeded(normalErr) {
		t.Error("expected false, got true")
	}
}

func TestGetAttempts(t *testing.T) {
	multiErr := &MultiError{
		Errors:   []error{errors.New("error")},
		Attempts: 5,
	}
	
	attempts := GetAttempts(multiErr)
	if attempts != 5 {
		t.Errorf("expected 5, got %d", attempts)
	}
	
	normalErr := errors.New("normal error")
	attempts = GetAttempts(normalErr)
	if attempts != 0 {
		t.Errorf("expected 0, got %d", attempts)
	}
}

func TestGetAllErrors(t *testing.T) {
	err1 := errors.New("error 1")
	err2 := errors.New("error 2")
	
	multiErr := &MultiError{
		Errors:   []error{err1, err2},
		Attempts: 2,
	}
	
	allErrors := GetAllErrors(multiErr)
	if len(allErrors) != 2 {
		t.Errorf("expected 2 errors, got %d", len(allErrors))
	}
	
	normalErr := errors.New("normal error")
	allErrors = GetAllErrors(normalErr)
	if allErrors != nil {
		t.Errorf("expected nil, got %v", allErrors)
	}
}

// ============================================================
// Benchmark
// ============================================================

func BenchmarkDo_Success(b *testing.B) {
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Do(ctx, func() error {
			return nil
		})
	}
}

func BenchmarkDo_Retry3Times(b *testing.B) {
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		called := 0
		Do(ctx, func() error {
			called++
			if called < 3 {
				return errors.New("temp error")
			}
			return nil
		}, Backoff(NoBackoff()))
	}
}

func BenchmarkDoWithData_Success(b *testing.B) {
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DoWithData(ctx, func() (int, error) {
			return 42, nil
		})
	}
}

