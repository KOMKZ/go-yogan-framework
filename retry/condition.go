package retry

import (
	"context"
	"errors"
	"net"
	"syscall"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RetryCondition 重试条件接口
type RetryCondition interface {
	// ShouldRetry 判断是否应该重试
	// err: 当前错误
	// attempt: 当前是第几次尝试（从 1 开始）
	// 返回 true 表示应该重试，false 表示不应该重试
	ShouldRetry(err error, attempt int) bool
}

// ============================================================
// 基础条件
// ============================================================

// alwaysRetry 总是重试
type alwaysRetry struct{}

// AlwaysRetry 创建总是重试的条件
func AlwaysRetry() RetryCondition {
	return &alwaysRetry{}
}

func (c *alwaysRetry) ShouldRetry(err error, attempt int) bool {
	return err != nil
}

// neverRetry 从不重试
type neverRetry struct{}

// NeverRetry 创建从不重试的条件
func NeverRetry() RetryCondition {
	return &neverRetry{}
}

func (c *neverRetry) ShouldRetry(err error, attempt int) bool {
	return false
}

// ============================================================
// 错误匹配条件
// ============================================================

// retryOnError 特定错误重试
type retryOnError struct {
	target error
}

// RetryOnError 创建特定错误重试条件（使用 errors.Is 判断）
func RetryOnError(target error) RetryCondition {
	return &retryOnError{target: target}
}

func (c *retryOnError) ShouldRetry(err error, attempt int) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, c.target)
}

// retryOnErrors 多个错误重试
type retryOnErrors struct {
	targets []error
}

// RetryOnErrors 创建多个错误重试条件
func RetryOnErrors(targets ...error) RetryCondition {
	return &retryOnErrors{targets: targets}
}

func (c *retryOnErrors) ShouldRetry(err error, attempt int) bool {
	if err == nil {
		return false
	}
	
	for _, target := range c.targets {
		if errors.Is(err, target) {
			return true
		}
	}
	
	return false
}

// ============================================================
// 自定义条件
// ============================================================

// retryOnCondition 自定义条件
type retryOnCondition struct {
	fn func(error) bool
}

// RetryOnCondition 创建自定义条件
func RetryOnCondition(fn func(error) bool) RetryCondition {
	return &retryOnCondition{fn: fn}
}

func (c *retryOnCondition) ShouldRetry(err error, attempt int) bool {
	if err == nil {
		return false
	}
	return c.fn(err)
}

// ============================================================
// gRPC 条件
// ============================================================

// retryOnGRPCCodes gRPC 状态码条件
type retryOnGRPCCodes struct {
	codes map[codes.Code]struct{}
}

// RetryOnGRPCCodes 创建 gRPC 状态码重试条件
func RetryOnGRPCCodes(targetCodes ...codes.Code) RetryCondition {
	codesMap := make(map[codes.Code]struct{}, len(targetCodes))
	for _, code := range targetCodes {
		codesMap[code] = struct{}{}
	}
	
	return &retryOnGRPCCodes{codes: codesMap}
}

func (c *retryOnGRPCCodes) ShouldRetry(err error, attempt int) bool {
	if err == nil {
		return false
	}
	
	// 尝试从错误中提取 gRPC 状态码
	st, ok := status.FromError(err)
	if !ok {
		return false
	}
	
	_, shouldRetry := c.codes[st.Code()]
	return shouldRetry
}

// ============================================================
// HTTP 条件
// ============================================================

// HTTPError HTTP 错误（需要应用层定义）
type HTTPError interface {
	error
	StatusCode() int
}

// retryOnHTTPStatus HTTP 状态码条件
type retryOnHTTPStatus struct {
	statuses map[int]struct{}
}

// RetryOnHTTPStatus 创建 HTTP 状态码重试条件
func RetryOnHTTPStatus(statuses ...int) RetryCondition {
	statusMap := make(map[int]struct{}, len(statuses))
	for _, status := range statuses {
		statusMap[status] = struct{}{}
	}
	
	return &retryOnHTTPStatus{statuses: statusMap}
}

func (c *retryOnHTTPStatus) ShouldRetry(err error, attempt int) bool {
	if err == nil {
		return false
	}
	
	// 尝试转换为 HTTPError
	httpErr, ok := err.(HTTPError)
	if !ok {
		return false
	}
	
	_, shouldRetry := c.statuses[httpErr.StatusCode()]
	return shouldRetry
}

// ============================================================
// 临时错误条件
// ============================================================

// temporaryError 临时错误接口（标准库）
type temporaryError interface {
	Temporary() bool
}

// retryOnTemporaryError 临时错误条件
type retryOnTemporaryError struct{}

// RetryOnTemporaryError 创建临时错误重试条件
// 包括：
// - 网络错误（net.Error 的 Temporary() 为 true）
// - Context 超时/取消
// - 连接拒绝/重置
func RetryOnTemporaryError() RetryCondition {
	return &retryOnTemporaryError{}
}

func (c *retryOnTemporaryError) ShouldRetry(err error, attempt int) bool {
	if err == nil {
		return false
	}
	
	// 1. 检查是否实现了 Temporary() 接口
	if te, ok := err.(temporaryError); ok && te.Temporary() {
		return true
	}
	
	// 2. Context 错误（超时/取消）- 认为是临时错误
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	
	// 3. 网络连接错误
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Temporary() || netErr.Timeout()
	}
	
	// 4. 常见的系统调用错误（wrapped in net.OpError）
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if opErr.Err != nil {
			if errors.Is(opErr.Err, syscall.ECONNREFUSED) ||
				errors.Is(opErr.Err, syscall.ECONNRESET) ||
				errors.Is(opErr.Err, syscall.ETIMEDOUT) ||
				errors.Is(opErr.Err, syscall.EPIPE) {
				return true
			}
		}
	}
	
	// 5. 直接的系统调用错误
	if errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.ETIMEDOUT) ||
		errors.Is(err, syscall.EPIPE) {
		return true
	}
	
	return false
}

// ============================================================
// 组合条件
// ============================================================

// andCondition AND 组合条件（所有条件都满足才重试）
type andCondition struct {
	conditions []RetryCondition
}

// And 创建 AND 组合条件
func And(conditions ...RetryCondition) RetryCondition {
	return &andCondition{conditions: conditions}
}

func (c *andCondition) ShouldRetry(err error, attempt int) bool {
	for _, cond := range c.conditions {
		if !cond.ShouldRetry(err, attempt) {
			return false
		}
	}
	return true
}

// orCondition OR 组合条件（任一条件满足就重试）
type orCondition struct {
	conditions []RetryCondition
}

// Or 创建 OR 组合条件
func Or(conditions ...RetryCondition) RetryCondition {
	return &orCondition{conditions: conditions}
}

func (c *orCondition) ShouldRetry(err error, attempt int) bool {
	for _, cond := range c.conditions {
		if cond.ShouldRetry(err, attempt) {
			return true
		}
	}
	return false
}

// notCondition NOT 条件（取反）
type notCondition struct {
	condition RetryCondition
}

// Not 创建 NOT 条件
func Not(condition RetryCondition) RetryCondition {
	return &notCondition{condition: condition}
}

func (c *notCondition) ShouldRetry(err error, attempt int) bool {
	return !c.condition.ShouldRetry(err, attempt)
}

