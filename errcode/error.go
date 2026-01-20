// Package errcode provides the basic types and functionalities for hierarchical error codes
// Error code format: MMBBBB (MM = module code 2 digits, B BBBB = business code 4 digits)
package errcode

import (
	"fmt"
	"net/http"
)

// LayeredError hierarchical error code
// Supports: error chaining, dynamic messages, context data, HTTP status code mapping, internationalization (message keys)
type LayeredError struct {
	module     string                 // Module name (user, order, payment)
	code       int                    // Complete error code (MMBBBB, e.g., 100001)
	msgKey     string                 // Message key (for internationalization, e.g., "error.user.not_found")
	msg        string                 // Default message (Chinese)
	httpStatus int                    // HTTP status code
	data       map[string]interface{} // context data
	cause      error                  // Original error (error chain)
}

// New Create hierarchical error codes
// moduleCode: Module code (10-99)
// businessCode: Business Code (0001-9999)
// module: module name (user, order, payment)
// msgKey: message key (for internationalization)
// msg: Default message
// httpStatus: HTTP status code (optional, default is 200)
func New(moduleCode, businessCode int, module, msgKey, msg string, httpStatus ...int) *LayeredError {
	code := moduleCode*10000 + businessCode
	status := http.StatusOK
	if len(httpStatus) > 0 {
		status = httpStatus[0]
	}
	return &LayeredError{
		module:     module,
		code:       code,
		msgKey:     msgKey,
		msg:        msg,
		httpStatus: status,
		data:       make(map[string]interface{}),
	}
}

// Implement error interface
func (e *LayeredError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", e.msg, e.cause)
	}
	return e.msg
}

// Code gets error code
func (e *LayeredError) Code() int {
	return e.code
}

// Module Get module name
func (e *LayeredError) Module() string {
	return e.module
}

// MsgKey retrieves the message key (for internationalization)
func (e *LayeredError) MsgKey() string {
	return e.msgKey
}

// Get error message
func (e *LayeredError) Message() string {
	return e.msg
}

// Get HTTP status code
func (e *LayeredError) HTTPStatus() int {
	return e.httpStatus
}

// Retrieve context data
func (e *LayeredError) Data() map[string]interface{} {
	return e.data
}

// Cause get original error
func (e *LayeredError) Cause() error {
	return e.cause
}

// Unwrap supports Go 1.13+ error chains
func (e *LayeredError) Unwrap() error {
	return e.cause
}

// WithMsg replace error message (return new instance, do not modify original instance)
func (e *LayeredError) WithMsg(msg string) *LayeredError {
	clone := *e
	clone.msg = msg
	return &clone
}

// WithMsgf format replacement error message (return new instance)
func (e *LayeredError) WithMsgf(format string, args ...interface{}) *LayeredError {
	clone := *e
	clone.msg = fmt.Sprintf(format, args...)
	return &clone
}

// WithData add single context data (return new instance)
func (e *LayeredError) WithData(key string, value interface{}) *LayeredError {
	clone := *e
	clone.data = e.cloneData()
	clone.data[key] = value
	return &clone
}

// WithFields batch add context data (return new instance)
func (e *LayeredError) WithFields(fields map[string]interface{}) *LayeredError {
	clone := *e
	clone.data = e.cloneData()
	for k, v := range fields {
		clone.data[k] = v
	}
	return &clone
}

// Wrap Wraps the original error (returns a new instance)
func (e *LayeredError) Wrap(cause error) *LayeredError {
	if cause == nil {
		return e
	}
	clone := *e
	clone.cause = cause
	return &clone
}

// Wrap the original error and format the message (return a new instance)
func (e *LayeredError) Wrapf(cause error, format string, args ...interface{}) *LayeredError {
	if cause == nil {
		return e.WithMsgf(format, args...)
	}
	clone := *e
	clone.cause = cause
	clone.msg = fmt.Sprintf(format, args...)
	return &clone
}

// Implements support for errors.Is() (by checking equality through code)
func (e *LayeredError) Is(target error) bool {
	t, ok := target.(*LayeredError)
	if !ok {
		return false
	}
	return e.code == t.code
}

// cloneData Clone context data (deep copy)
func (e *LayeredError) cloneData() map[string]interface{} {
	data := make(map[string]interface{}, len(e.data))
	for k, v := range e.data {
		data[k] = v
	}
	return data
}

// Set HTTP status code (return new instance)
func (e *LayeredError) WithHTTPStatus(status int) *LayeredError {
	clone := *e
	clone.httpStatus = status
	return &clone
}

// String returns an erroneous string representation (for debugging)
func (e *LayeredError) String() string {
	if e.cause != nil {
		return fmt.Sprintf("LayeredError{code:%d, module:%s, msg:%s, cause:%v}",
			e.code, e.module, e.msg, e.cause)
	}
	return fmt.Sprintf("LayeredError{code:%d, module:%s, msg:%s}",
		e.code, e.module, e.msg)
}

