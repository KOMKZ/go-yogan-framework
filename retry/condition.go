package retry

import (
	"context"
	"errors"
	"net"
	"syscall"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RetryCondition retry condition interface
type RetryCondition interface {
	// ShouldRetry determines whether a retry should be performed
	// err: current error
	// attempt: current retry count (starting from 1)
	// return true to indicate a retry should be performed, false otherwise
	ShouldRetry(err error, attempt int) bool
}

// ============================================================
// Basic conditions
// ============================================================

// alwaysRetry Always retry
type alwaysRetry struct{}

// AlwaysRetry creates the condition for always retrying
func AlwaysRetry() RetryCondition {
	return &alwaysRetry{}
}

func (c *alwaysRetry) ShouldRetry(err error, attempt int) bool {
	return err != nil
}

// neverRetry never retry
type neverRetry struct{}

// NeverRetry creates conditions for never retrying
func NeverRetry() RetryCondition {
	return &neverRetry{}
}

func (c *neverRetry) ShouldRetry(err error, attempt int) bool {
	return false
}

// ============================================================
// Error matching conditions
// ============================================================

// retryOnSpecificError
type retryOnError struct {
	target error
}

// Create specific error retry conditions (use errors.Is to check)
func RetryOnError(target error) RetryCondition {
	return &retryOnError{target: target}
}

func (c *retryOnError) ShouldRetry(err error, attempt int) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, c.target)
}

// retryOnErrors retry on multiple errors
type retryOnErrors struct {
	targets []error
}

// Create multiple error retry conditions
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
// Custom condition
// ============================================================

// custom condition for retrying
type retryOnCondition struct {
	fn func(error) bool
}

// Create custom condition for retry
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
// gRPC condition
// ============================================================

// retryOnGRPCCodes gRPC status code conditions
type retryOnGRPCCodes struct {
	codes map[codes.Code]struct{}
}

// Create gRPC status code retry conditions
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
	
	// Try to extract gRPC status code from error
	st, ok := status.FromError(err)
	if !ok {
		return false
	}
	
	_, shouldRetry := c.codes[st.Code()]
	return shouldRetry
}

// ============================================================
// HTTP conditions
// ============================================================

// HTTPError HTTP error (requires definition at the application layer)
type HTTPError interface {
	error
	StatusCode() int
}

// retry on HTTP status code condition
type retryOnHTTPStatus struct {
	statuses map[int]struct{}
}

// Create HTTP status code retry conditions
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
	
	// Try to convert to HTTPError
	httpErr, ok := err.(HTTPError)
	if !ok {
		return false
	}
	
	_, shouldRetry := c.statuses[httpErr.StatusCode()]
	return shouldRetry
}

// ============================================================
// temporary error condition
// ============================================================

// temporaryError Temporary error interface (standard library)
type temporaryError interface {
	Temporary() bool
}

// retryOnTemporaryError temporary error condition
type retryOnTemporaryError struct{}

// RetryOnTemporaryError Create retry condition for temporary errors
// Includes:
// - Network error (net.Error's Temporary() is true)
// - Timeout/Cancellation Context
// - connection refused/reset
func RetryOnTemporaryError() RetryCondition {
	return &retryOnTemporaryError{}
}

func (c *retryOnTemporaryError) ShouldRetry(err error, attempt int) bool {
	if err == nil {
		return false
	}
	
	// Check if the Temporary() interface is implemented
	if te, ok := err.(temporaryError); ok && te.Temporary() {
		return true
	}
	
	// 2. Context errors (timeout/cancel) - considered transient errors
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	
	// Network connection error
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Temporary() || netErr.Timeout()
	}
	
	// 4. Common system call errors (wrapped in net.OpError)
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
	
	// 5. Direct system call errors
	if errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.ETIMEDOUT) ||
		errors.Is(err, syscall.EPIPE) {
		return true
	}
	
	return false
}

// ============================================================
// Combination condition
// ============================================================

// andCondition AND combine conditions (all conditions must be met to retry)
type andCondition struct {
	conditions []RetryCondition
}

// And create AND combination conditions
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

// orCondition OR combined condition (retry if any condition is met)
type orCondition struct {
	conditions []RetryCondition
}

// Or create OR combined conditions
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

// notCondition NOT condition (negate)
type notCondition struct {
	condition RetryCondition
}

// Do not create NOT conditions
func Not(condition RetryCondition) RetryCondition {
	return &notCondition{condition: condition}
}

func (c *notCondition) ShouldRetry(err error, attempt int) bool {
	return !c.condition.ShouldRetry(err, attempt)
}

