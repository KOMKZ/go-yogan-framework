package retry

import (
	"context"
	"errors"
	"time"
)

// Do 执行操作，失败时重试
// 返回最后一次的错误（如果所有尝试都失败）
func Do(ctx context.Context, operation func() error, opts ...Option) error {
	_, err := DoWithData(ctx, func() (struct{}, error) {
		return struct{}{}, operation()
	}, opts...)
	
	return err
}

// DoWithData 执行操作并返回数据，失败时重试
// 泛型支持，返回业务数据 + 错误
func DoWithData[T any](ctx context.Context, operation func() (T, error), opts ...Option) (T, error) {
	// 加载配置
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}
	
	var result T
	var errs []error
	
	for attempt := 1; attempt <= cfg.maxAttempts; attempt++ {
		// 检查 Context 是否已取消或超时
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}
		
		// 检查重试预算（如果启用）
		if cfg.budget != nil && attempt > 1 && !cfg.budget.Allow() {
			// 预算耗尽，返回错误
			multiErr := &MultiError{
				Errors:   append(errs, ErrBudgetExhausted),
				Attempts: attempt - 1,
			}
			return result, multiErr
		}
		
		// 执行操作（带超时控制）
		var err error
		if cfg.timeout > 0 {
			// 有单次超时限制
			opCtx, cancel := context.WithTimeout(ctx, cfg.timeout)
			result, err = executeWithContext(opCtx, operation)
			cancel()
		} else {
			// 无超时限制，直接执行
			result, err = operation()
		}
		
		// 成功，返回结果
		if err == nil {
			// 记录成功（用于预算统计）
			if cfg.budget != nil {
				cfg.budget.Record(true)
			}
			return result, nil
		}
		
		// 失败，记录错误
		errs = append(errs, err)
		
		// 记录失败（用于预算统计）
		if cfg.budget != nil {
			cfg.budget.Record(false)
		}
		
		// 判断是否应该重试
		if !cfg.condition.ShouldRetry(err, attempt) {
			// 不应该重试，直接返回
			multiErr := &MultiError{
				Errors:   errs,
				Attempts: attempt,
			}
			return result, multiErr
		}
		
		// 最后一次尝试，不再等待
		if attempt == cfg.maxAttempts {
			multiErr := &MultiError{
				Errors:   errs,
				Attempts: attempt,
			}
			return result, multiErr
		}
		
		// 触发重试回调
		if cfg.onRetry != nil {
			cfg.onRetry(attempt, err)
		}
		
		// 计算退避时间
		backoff := cfg.backoff.Next(attempt)
		
		// 检查剩余时间是否足够（如果有 Context Deadline）
		if deadline, ok := ctx.Deadline(); ok {
			remaining := time.Until(deadline)
			if remaining < backoff {
				// 时间不足，停止重试
				multiErr := &MultiError{
					Errors:   append(errs, context.DeadlineExceeded),
					Attempts: attempt,
				}
				return result, multiErr
			}
		}
		
		// 等待退避时间（可被 Context 取消）
		select {
		case <-time.After(backoff):
			// 继续重试
		case <-ctx.Done():
			return result, ctx.Err()
		}
	}
	
	// 理论上不会到达这里
	multiErr := &MultiError{
		Errors:   errs,
		Attempts: cfg.maxAttempts,
	}
	return result, multiErr
}

// executeWithContext 在带超时的 Context 中执行操作
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
// 辅助函数
// ============================================================

// IsMaxAttemptsExceeded 判断是否因为超过最大重试次数而失败
func IsMaxAttemptsExceeded(err error) bool {
	var multiErr *MultiError
	if errors.As(err, &multiErr) {
		return multiErr.Attempts > 0
	}
	return false
}

// GetAttempts 获取重试次数
func GetAttempts(err error) int {
	var multiErr *MultiError
	if errors.As(err, &multiErr) {
		return multiErr.Attempts
	}
	return 0
}

// GetAllErrors 获取所有错误
func GetAllErrors(err error) []error {
	var multiErr *MultiError
	if errors.As(err, &multiErr) {
		return multiErr.Errors
	}
	return nil
}

