// Package errcode 提供分层错误码的基础类型和功能
// 错误码格式：MMBBBB（MM=模块码2位，BBBB=业务码4位）
package errcode

import (
	"fmt"
	"net/http"
)

// LayeredError 分层错误码
// 支持：错误链、动态消息、上下文数据、HTTP状态码映射、国际化（消息键）
type LayeredError struct {
	module     string                 // 模块名（user, order, payment）
	code       int                    // 完整错误码（MMBBBB，如 100001）
	msgKey     string                 // 消息键（用于国际化，如 "error.user.not_found"）
	msg        string                 // 默认消息（中文）
	httpStatus int                    // HTTP 状态码
	data       map[string]interface{} // 上下文数据
	cause      error                  // 原始错误（错误链）
}

// New 创建分层错误码
// moduleCode: 模块码（10-99）
// businessCode: 业务码（0001-9999）
// module: 模块名（user, order, payment）
// msgKey: 消息键（用于国际化）
// msg: 默认消息
// httpStatus: HTTP 状态码（可选，默认 200）
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

// Error 实现 error 接口
func (e *LayeredError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", e.msg, e.cause)
	}
	return e.msg
}

// Code 获取错误码
func (e *LayeredError) Code() int {
	return e.code
}

// Module 获取模块名
func (e *LayeredError) Module() string {
	return e.module
}

// MsgKey 获取消息键（用于国际化）
func (e *LayeredError) MsgKey() string {
	return e.msgKey
}

// Message 获取错误消息
func (e *LayeredError) Message() string {
	return e.msg
}

// HTTPStatus 获取 HTTP 状态码
func (e *LayeredError) HTTPStatus() int {
	return e.httpStatus
}

// Data 获取上下文数据
func (e *LayeredError) Data() map[string]interface{} {
	return e.data
}

// Cause 获取原始错误
func (e *LayeredError) Cause() error {
	return e.cause
}

// Unwrap 支持 Go 1.13+ 错误链
func (e *LayeredError) Unwrap() error {
	return e.cause
}

// WithMsg 替换错误消息（返回新实例，不修改原实例）
func (e *LayeredError) WithMsg(msg string) *LayeredError {
	clone := *e
	clone.msg = msg
	return &clone
}

// WithMsgf 格式化替换错误消息（返回新实例）
func (e *LayeredError) WithMsgf(format string, args ...interface{}) *LayeredError {
	clone := *e
	clone.msg = fmt.Sprintf(format, args...)
	return &clone
}

// WithData 添加单个上下文数据（返回新实例）
func (e *LayeredError) WithData(key string, value interface{}) *LayeredError {
	clone := *e
	clone.data = e.cloneData()
	clone.data[key] = value
	return &clone
}

// WithFields 批量添加上下文数据（返回新实例）
func (e *LayeredError) WithFields(fields map[string]interface{}) *LayeredError {
	clone := *e
	clone.data = e.cloneData()
	for k, v := range fields {
		clone.data[k] = v
	}
	return &clone
}

// Wrap 包装原始错误（返回新实例）
func (e *LayeredError) Wrap(cause error) *LayeredError {
	if cause == nil {
		return e
	}
	clone := *e
	clone.cause = cause
	return &clone
}

// Wrapf 包装原始错误并格式化消息（返回新实例）
func (e *LayeredError) Wrapf(cause error, format string, args ...interface{}) *LayeredError {
	if cause == nil {
		return e.WithMsgf(format, args...)
	}
	clone := *e
	clone.cause = cause
	clone.msg = fmt.Sprintf(format, args...)
	return &clone
}

// Is 实现 errors.Is() 支持（通过 code 判断相等）
func (e *LayeredError) Is(target error) bool {
	t, ok := target.(*LayeredError)
	if !ok {
		return false
	}
	return e.code == t.code
}

// cloneData 克隆上下文数据（深拷贝）
func (e *LayeredError) cloneData() map[string]interface{} {
	data := make(map[string]interface{}, len(e.data))
	for k, v := range e.data {
		data[k] = v
	}
	return data
}

// WithHTTPStatus 设置 HTTP 状态码（返回新实例）
func (e *LayeredError) WithHTTPStatus(status int) *LayeredError {
	clone := *e
	clone.httpStatus = status
	return &clone
}

// String 返回错误的字符串表示（用于调试）
func (e *LayeredError) String() string {
	if e.cause != nil {
		return fmt.Sprintf("LayeredError{code:%d, module:%s, msg:%s, cause:%v}",
			e.code, e.module, e.msg, e.cause)
	}
	return fmt.Sprintf("LayeredError{code:%d, module:%s, msg:%s}",
		e.code, e.module, e.msg)
}

