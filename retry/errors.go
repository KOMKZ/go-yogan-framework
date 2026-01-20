package retry

import (
	"errors"
	"fmt"
	"strings"
)

// MultiError aggregation of failures from multiple retry attempts
type MultiError struct {
	Errors   []error // All attempted errors
	Attempts int     // attempt count
}

// Implement error interface
func (e *MultiError) Error() string {
	if len(e.Errors) == 0 {
		return "retry failed: no errors"
	}
	
	// Return the last error
	return e.Errors[len(e.Errors)-1].Error()
}

// Unwrap implements the errors.Unwrap interface (returns the innermost error)
func (e *MultiError) Unwrap() error {
	if len(e.Errors) == 0 {
		return nil
	}
	return e.Errors[len(e.Errors)-1]
}

// Returns the string representation of all errors
func (e *MultiError) AllErrors() string {
	if len(e.Errors) == 0 {
		return ""
	}
	
	var b strings.Builder
	b.WriteString(fmt.Sprintf("retry failed after %d attempts:", e.Attempts))
	for i, err := range e.Errors {
		b.WriteString(fmt.Sprintf("\n  attempt %d: %v", i+1, err))
	}
	
	return b.String()
}

// Returns the last error
func (e *MultiError) LastError() error {
	if len(e.Errors) == 0 {
		return nil
	}
	return e.Errors[len(e.Errors)-1]
}

// Returns the first error
func (e *MultiError) FirstError() error {
	if len(e.Errors) == 0 {
		return nil
	}
	return e.Errors[0]
}

// ============================================================
// Predefined error
// ============================================================

var (
	// ErrMaxAttemptsExceeded Maximum retry attempts exceeded
	ErrMaxAttemptsExceeded = errors.New("retry: max attempts exceeded")
	
	// ErrBudgetExhausted retry budget exhausted
	ErrBudgetExhausted = errors.New("retry: budget exhausted")
)

