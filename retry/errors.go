package retry

import (
	"errors"
	"fmt"
	"strings"
)

// MultiError 多次重试失败的错误聚合
type MultiError struct {
	Errors   []error // 所有尝试的错误
	Attempts int     // 尝试次数
}

// Error 实现 error 接口
func (e *MultiError) Error() string {
	if len(e.Errors) == 0 {
		return "retry failed: no errors"
	}
	
	// 返回最后一次的错误
	return e.Errors[len(e.Errors)-1].Error()
}

// Unwrap 实现 errors.Unwrap 接口（返回最后一次错误）
func (e *MultiError) Unwrap() error {
	if len(e.Errors) == 0 {
		return nil
	}
	return e.Errors[len(e.Errors)-1]
}

// AllErrors 返回所有错误的字符串表示
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

// LastError 返回最后一次的错误
func (e *MultiError) LastError() error {
	if len(e.Errors) == 0 {
		return nil
	}
	return e.Errors[len(e.Errors)-1]
}

// FirstError 返回第一次的错误
func (e *MultiError) FirstError() error {
	if len(e.Errors) == 0 {
		return nil
	}
	return e.Errors[0]
}

// ============================================================
// 预定义错误
// ============================================================

var (
	// ErrMaxAttemptsExceeded 超过最大重试次数
	ErrMaxAttemptsExceeded = errors.New("retry: max attempts exceeded")
	
	// ErrBudgetExhausted 重试预算耗尽
	ErrBudgetExhausted = errors.New("retry: budget exhausted")
)

