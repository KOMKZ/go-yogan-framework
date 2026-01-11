package errcode

import (
	"errors"
	"net/http"
	"testing"
)

// TestLayeredError_New 测试创建分层错误码
func TestLayeredError_New(t *testing.T) {
	err := New(10, 1, "user", "error.user.not_found", "User not found")

	if err.Code() != 100001 {
		t.Errorf("expected code 100001, got %d", err.Code())
	}
	if err.Module() != "user" {
		t.Errorf("expected module 'user', got %s", err.Module())
	}
	if err.MsgKey() != "error.user.not_found" {
		t.Errorf("expected msgKey 'error.user.not_found', got %s", err.MsgKey())
	}
	if err.Message() != "User not found" {
		t.Errorf("expected msg '用户不存在', got %s", err.Message())
	}
	if err.HTTPStatus() != http.StatusOK {
		t.Errorf("expected httpStatus 200, got %d", err.HTTPStatus())
	}
}

// TestLayeredError_New_WithHTTPStatus 测试创建错误码并指定 HTTP 状态码
func TestLayeredError_New_WithHTTPStatus(t *testing.T) {
	err := New(10, 1, "user", "error.user.not_found", "User not found", http.StatusNotFound)

	if err.HTTPStatus() != http.StatusNotFound {
		t.Errorf("expected httpStatus 404, got %d", err.HTTPStatus())
	}
}

// TestLayeredError_Error 测试 error 接口实现
func TestLayeredError_Error(t *testing.T) {
	err := New(10, 1, "user", "error.user.not_found", "User not found")

	if err.Error() != "User not found" {
		t.Errorf("expected error message '用户不存在', got %s", err.Error())
	}
}

// TestLayeredError_Error_WithCause 测试 error 接口实现（带原始错误）
func TestLayeredError_Error_WithCause(t *testing.T) {
	originalErr := errors.New("database connection failed")
	err := New(10, 1, "user", "error.user.not_found", "User not found").Wrap(originalErr)

	expected := "用户不存在: database connection failed"
	if err.Error() != expected {
		t.Errorf("expected error message '%s', got %s", expected, err.Error())
	}
}

// TestLayeredError_WithMsg 测试动态消息
func TestLayeredError_WithMsg(t *testing.T) {
	original := New(10, 1, "user", "error.user.not_found", "User not found")
	modified := original.WithMsg("用户未找到")

	// 原实例不变
	if original.Message() != "User not found" {
		t.Errorf("original message should not change, got %s", original.Message())
	}

	// 新实例消息已改变
	if modified.Message() != "用户未找到" {
		t.Errorf("expected modified message '用户未找到', got %s", modified.Message())
	}

	// 错误码不变
	if modified.Code() != 100001 {
		t.Errorf("code should not change, got %d", modified.Code())
	}
}

// TestLayeredError_WithMsgf 测试格式化动态消息
func TestLayeredError_WithMsgf(t *testing.T) {
	err := New(10, 1, "user", "error.user.not_found", "User not found")
	modified := err.WithMsgf("用户 %d 不存在", 123)

	expected := "用户 123 不存在"
	if modified.Message() != expected {
		t.Errorf("expected message '%s', got %s", expected, modified.Message())
	}
}

// TestLayeredError_WithData 测试添加单个上下文数据
func TestLayeredError_WithData(t *testing.T) {
	original := New(10, 1, "user", "error.user.not_found", "User not found")
	modified := original.WithData("user_id", 123)

	// 原实例不变
	if len(original.Data()) != 0 {
		t.Errorf("original data should be empty, got %d items", len(original.Data()))
	}

	// 新实例有数据
	if len(modified.Data()) != 1 {
		t.Errorf("expected 1 data item, got %d", len(modified.Data()))
	}
	if modified.Data()["user_id"] != 123 {
		t.Errorf("expected user_id=123, got %v", modified.Data()["user_id"])
	}
}

// TestLayeredError_WithFields 测试批量添加上下文数据
func TestLayeredError_WithFields(t *testing.T) {
	err := New(10, 1, "user", "error.user.not_found", "User not found")
	modified := err.WithFields(map[string]interface{}{
		"user_id": 123,
		"email":   "test@example.com",
	})

	if len(modified.Data()) != 2 {
		t.Errorf("expected 2 data items, got %d", len(modified.Data()))
	}
	if modified.Data()["user_id"] != 123 {
		t.Errorf("expected user_id=123, got %v", modified.Data()["user_id"])
	}
	if modified.Data()["email"] != "test@example.com" {
		t.Errorf("expected email=test@example.com, got %v", modified.Data()["email"])
	}
}

// TestLayeredError_Wrap 测试包装原始错误
func TestLayeredError_Wrap(t *testing.T) {
	originalErr := errors.New("database connection failed")
	err := New(10, 1, "user", "error.user.not_found", "User not found")
	wrapped := err.Wrap(originalErr)

	if wrapped.Cause() != originalErr {
		t.Errorf("expected cause to be %v, got %v", originalErr, wrapped.Cause())
	}

	// 测试 Unwrap
	if errors.Unwrap(wrapped) != originalErr {
		t.Errorf("expected Unwrap to return %v, got %v", originalErr, errors.Unwrap(wrapped))
	}
}

// TestLayeredError_Wrap_Nil 测试包装 nil 错误
func TestLayeredError_Wrap_Nil(t *testing.T) {
	err := New(10, 1, "user", "error.user.not_found", "User not found")
	wrapped := err.Wrap(nil)

	if wrapped != err {
		t.Errorf("wrapping nil should return original error")
	}
}

// TestLayeredError_Wrapf 测试包装错误并格式化消息
func TestLayeredError_Wrapf(t *testing.T) {
	originalErr := errors.New("database connection failed")
	err := New(10, 1, "user", "error.user.not_found", "User not found")
	wrapped := err.Wrapf(originalErr, "查询用户 %d 失败", 123)

	if wrapped.Cause() != originalErr {
		t.Errorf("expected cause to be %v, got %v", originalErr, wrapped.Cause())
	}

	expected := "查询用户 123 失败"
	if wrapped.Message() != expected {
		t.Errorf("expected message '%s', got %s", expected, wrapped.Message())
	}
}

// TestLayeredError_Is 测试 errors.Is() 支持
func TestLayeredError_Is(t *testing.T) {
	err1 := New(10, 1, "user", "error.user.not_found", "User not found")
	err2 := New(10, 1, "user", "error.user.not_found", "User not found")
	err3 := New(10, 2, "user", "error.user.exists", "用户已存在")

	// 相同错误码
	if !errors.Is(err1, err2) {
		t.Errorf("err1 and err2 should be equal")
	}

	// 不同错误码
	if errors.Is(err1, err3) {
		t.Errorf("err1 and err3 should not be equal")
	}

	// 动态消息不影响相等性
	err4 := err1.WithMsg("用户未找到")
	if !errors.Is(err1, err4) {
		t.Errorf("err1 and err4 should be equal (code is the same)")
	}
}

// TestLayeredError_Is_WithCause 测试错误链中的 errors.Is()
func TestLayeredError_Is_WithCause(t *testing.T) {
	originalErr := errors.New("database connection failed")
	err := New(10, 1, "user", "error.user.not_found", "User not found").Wrap(originalErr)

	// 错误码相等
	if !errors.Is(err, New(10, 1, "user", "error.user.not_found", "User not found")) {
		t.Errorf("should match by error code")
	}

	// 原始错误
	if !errors.Is(err, originalErr) {
		t.Errorf("should match original error in chain")
	}
}

// TestLayeredError_WithHTTPStatus 测试设置 HTTP 状态码
func TestLayeredError_WithHTTPStatus(t *testing.T) {
	err := New(10, 1, "user", "error.user.not_found", "User not found")
	modified := err.WithHTTPStatus(http.StatusNotFound)

	if err.HTTPStatus() != http.StatusOK {
		t.Errorf("original httpStatus should not change, got %d", err.HTTPStatus())
	}

	if modified.HTTPStatus() != http.StatusNotFound {
		t.Errorf("expected httpStatus 404, got %d", modified.HTTPStatus())
	}
}

// TestLayeredError_String 测试 String() 方法
func TestLayeredError_String(t *testing.T) {
	err := New(10, 1, "user", "error.user.not_found", "User not found")
	str := err.String()

	expected := "LayeredError{code:100001, module:user, msg:用户不存在}"
	if str != expected {
		t.Errorf("expected '%s', got '%s'", expected, str)
	}
}

// TestLayeredError_String_WithCause 测试 String() 方法（带原始错误）
func TestLayeredError_String_WithCause(t *testing.T) {
	originalErr := errors.New("database connection failed")
	err := New(10, 1, "user", "error.user.not_found", "User not found").Wrap(originalErr)
	str := err.String()

	expected := "LayeredError{code:100001, module:user, msg:用户不存在, cause:database connection failed}"
	if str != expected {
		t.Errorf("expected '%s', got '%s'", expected, str)
	}
}

// TestLayeredError_ChainOperations 测试链式操作
func TestLayeredError_ChainOperations(t *testing.T) {
	err := New(10, 1, "user", "error.user.not_found", "User not found").
		WithMsgf("用户 %d 不存在", 123).
		WithData("user_id", 123).
		WithData("email", "test@example.com").
		WithHTTPStatus(http.StatusNotFound)

	if err.Message() != "用户 123 不存在" {
		t.Errorf("expected message '用户 123 不存在', got %s", err.Message())
	}
	if len(err.Data()) != 2 {
		t.Errorf("expected 2 data items, got %d", len(err.Data()))
	}
	if err.HTTPStatus() != http.StatusNotFound {
		t.Errorf("expected httpStatus 404, got %d", err.HTTPStatus())
	}
}

// TestLayeredError_ImmutableOriginal 测试原实例不可变性
func TestLayeredError_ImmutableOriginal(t *testing.T) {
	original := New(10, 1, "user", "error.user.not_found", "User not found")
	originalCode := original.Code()
	originalMsg := original.Message()
	originalDataLen := len(original.Data())

	// 各种修改操作
	_ = original.WithMsg("新消息")
	_ = original.WithData("key", "value")
	_ = original.WithHTTPStatus(http.StatusNotFound)
	_ = original.Wrap(errors.New("cause"))

	// 验证原实例未改变
	if original.Code() != originalCode {
		t.Errorf("original code changed")
	}
	if original.Message() != originalMsg {
		t.Errorf("original message changed")
	}
	if len(original.Data()) != originalDataLen {
		t.Errorf("original data changed")
	}
	if original.HTTPStatus() != http.StatusOK {
		t.Errorf("original httpStatus changed")
	}
	if original.Cause() != nil {
		t.Errorf("original cause changed")
	}
}

