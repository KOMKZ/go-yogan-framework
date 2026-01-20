package retry

import (
	"context"
	"errors"
	"time"
)

// Perform operation, retry on failure
// Return the last error (if all attempts fail)
func Do(ctx context.Context, operation func() error, opts ...Option) error {
	_, err := DoWithData(ctx, func() (struct{}, error) {
		return struct{}{}, operation()
	}, opts...)
	
	return err
}

// DoWithData performs operations and returns data, retries on failure
// Generic support, return business data + error
func DoWithData[T any](ctx context.Context, operation func() (T, error), opts ...Option) (T, error) {
	// Load configuration
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}
	
	var result T
	var errs []error
	
	for attempt := 1; attempt <= cfg.maxAttempts; attempt++ {
		// Check if the Context has been cancelled or timed out
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}
		
		// Check retry budget (if enabled)
		if cfg.budget != nil && attempt > 1 && !cfg.budget.Allow() {
			// budget exhausted, return error
			multiErr := &MultiError{
				Errors:   append(errs, ErrBudgetExhausted),
				Attempts: attempt - 1,
			}
			return result, multiErr
		}
		
		// Execute operation (with timeout control)
		var err error
		if cfg.timeout > 0 {
			// Has single request timeout limit
			opCtx, cancel := context.WithTimeout(ctx, cfg.timeout)
			result, err = executeWithContext(opCtx, operation)
			cancel()
		} else {
			// No timeout limit, execute directly
			result, err = operation()
		}
		
		// Success, return result
		if err == nil {
			// Log successful (for budget statistics)
			if cfg.budget != nil {
				cfg.budget.Record(true)
			}
			return result, nil
		}
		
		// failure, log error
		errs = append(errs, err)
		
		// Log failure (for budget statistics)
		if cfg.budget != nil {
			cfg.budget.Record(false)
		}
		
		// Determine if a retry should be attempted
		if !cfg.condition.ShouldRetry(err, attempt) {
			// Should not retry, return directly
			multiErr := &MultiError{
				Errors:   errs,
				Attempts: attempt,
			}
			return result, multiErr
		}
		
		// Final attempt, no more waiting
		if attempt == cfg.maxAttempts {
			multiErr := &MultiError{
				Errors:   errs,
				Attempts: attempt,
			}
			return result, multiErr
		}
		
		// trigger retry callback
		if cfg.onRetry != nil {
			cfg.onRetry(attempt, err)
		}
		
		// Calculate backoff time
		backoff := cfg.backoff.Next(attempt)
		
		// Check if the remaining time is sufficient (if Context Deadline exists)
		if deadline, ok := ctx.Deadline(); ok {
			remaining := time.Until(deadline)
			if remaining < backoff {
				// Time insufficient, stop retrying
				multiErr := &MultiError{
					Errors:   append(errs, context.DeadlineExceeded),
					Attempts: attempt,
				}
				return result, multiErr
			}
		}
		
		// wait for backoff time (can be canceled by Context)
		select {
		case <-time.After(backoff):
			// Continue retrying
		case <-ctx.Done():
			return result, ctx.Err()
		}
	}
	
	// Theoretically should not reach here
	multiErr := &MultiError{
		Errors:   errs,
		Attempts: cfg.maxAttempts,
	}
	return result, multiErr
}

// executeWithContext Execute the operation in a Context with timeout
func executeWithContext[T any](ctx context.Context, operation func() (T, error)) (T, error) {
	type result struct {
		data T
		err  error
	}
	
	ch := make(chan result, 1)
	
	go func() {
		data, err := operation()
		ch <- result{data: data, err: err}
	}()
	
	select {
	case res := <-ch:
		return res.data, res.err
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	}
}

// ============================================================
// helper function
// ============================================================

// Check if failure is due to exceeding maximum retry attempts
func IsMaxAttemptsExceeded(err error) bool {
	var multiErr *MultiError
	if errors.As(err, &multiErr) {
		return multiErr.Attempts > 0
	}
	return false
}

// GetAttempts Get retry attempts
func GetAttempts(err error) int {
	var multiErr *MultiError
	if errors.As(err, &multiErr) {
		return multiErr.Attempts
	}
	return 0
}

// GetAllErrors Get all errors
func GetAllErrors(err error) []error {
	var multiErr *MultiError
	if errors.As(err, &multiErr) {
		return multiErr.Errors
	}
	return nil
}

