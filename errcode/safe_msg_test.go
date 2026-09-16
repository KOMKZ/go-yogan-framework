package errcode

import (
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestSafeMessageNilReturnsEmpty(t *testing.T) {
	public, details := SafeMessage(nil)
	if public != "" {
		t.Fatalf("public should be empty for nil err, got %q", public)
	}
	if details != nil {
		t.Fatalf("details should be nil for nil err, got %v", details)
	}
}

func TestSafeMessageLayeredErrorExposesRegisteredMessage(t *testing.T) {
	le := New(25, 1001, "users", "error.users.not_found", "用户不存在", http.StatusNotFound).
		Wrap(Capture(errors.New("db: record not found"), "users.repo.find_by_id"))

	public, details := SafeMessage(le)

	if public != "用户不存在" {
		t.Fatalf("public = %q, want 用户不存在", public)
	}
	if details["code"] != 251001 {
		t.Fatalf("details.code = %v, want 251001", details["code"])
	}
	if details["module"] != "users" {
		t.Fatalf("details.module = %v, want users", details["module"])
	}
	if details["msg_key"] != "error.users.not_found" {
		t.Fatalf("details.msg_key = %v", details["msg_key"])
	}
	if details["operation"] != "users.repo.find_by_id" {
		t.Fatalf("details.operation = %v, want users.repo.find_by_id", details["operation"])
	}
	if details["origin_stack"] == "" {
		t.Fatalf("expected origin_stack to be populated")
	}
	if details["cause_type"] == nil {
		t.Fatalf("expected cause_type to be populated")
	}
	if details["root_type"] == nil {
		t.Fatalf("expected root_type to be populated")
	}
}

func TestSafeMessagePlainErrorReturnsFixedPublicMessage(t *testing.T) {
	public, details := SafeMessage(errors.New("internal: sensitive detail"))

	if public != "内部错误，请稍后再试" {
		t.Fatalf("public = %q, want 内部错误，请稍后再试", public)
	}
	if details["cause_type"] != "*errors.errorString" {
		t.Fatalf("details.cause_type = %v", details["cause_type"])
	}
	if details["cause_message"] != "internal: sensitive detail" {
		t.Fatalf("details.cause_message = %v", details["cause_message"])
	}
}

func TestSafeMessageDoesNotLeakRawErrorToPublic(t *testing.T) {
	// 关键回归测试：必须确保 err.Error() 不出现在 public 字段。
	secretErr := errors.New("dsn=postgres://user:pass@host:5432/db")
	public, _ := SafeMessage(secretErr)

	if strings.Contains(public, "dsn=") || strings.Contains(public, "postgres://") || strings.Contains(public, "pass") {
		t.Fatalf("SafeMessage leaked raw err.Error() to public: %q", public)
	}
}

func TestSafeMessageWrappedPlainErrorStillReturnsFixedMessage(t *testing.T) {
	wrapped := fmtWrap(errors.New("sdk timeout"))
	public, details := SafeMessage(wrapped)

	if public != "内部错误，请稍后再试" {
		t.Fatalf("public = %q, want 内部错误", public)
	}
	if details["cause_message"] != "sdk timeout" {
		t.Fatalf("details.cause_message = %v", details["cause_message"])
	}
}

func fmtWrap(err error) error {
	return err
}
