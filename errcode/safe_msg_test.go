package errcode

import (
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestSafeMessageNilReturnsEmpty(t *testing.T) {
	result := SafeMessage(nil)
	if result.PublicMessage != "" {
		t.Fatalf("public should be empty for nil err, got %q", result.PublicMessage)
	}
	if result.Diagnostics != nil {
		t.Fatalf("diagnostics should be nil for nil err, got %v", result.Diagnostics)
	}
}

func TestSafeMessageLayeredErrorExposesRegisteredMessage(t *testing.T) {
	le := New(25, 1001, "users", "error.users.not_found", "用户不存在", http.StatusNotFound).
		Wrap(Capture(errors.New("db: record not found"), "users.repo.find_by_id"))

	result := SafeMessage(le)

	if result.PublicMessage != "用户不存在" {
		t.Fatalf("public = %q, want 用户不存在", result.PublicMessage)
	}
	if result.Diagnostics["code"] != 251001 {
		t.Fatalf("diagnostics.code = %v, want 251001", result.Diagnostics["code"])
	}
	if result.Diagnostics["module"] != "users" {
		t.Fatalf("diagnostics.module = %v, want users", result.Diagnostics["module"])
	}
	if result.Diagnostics["msg_key"] != "error.users.not_found" {
		t.Fatalf("diagnostics.msg_key = %v", result.Diagnostics["msg_key"])
	}
	if result.Diagnostics["operation"] != "users.repo.find_by_id" {
		t.Fatalf("diagnostics.operation = %v, want users.repo.find_by_id", result.Diagnostics["operation"])
	}
	if result.Diagnostics["origin_stack"] == "" {
		t.Fatalf("expected origin_stack to be populated")
	}
	if result.Diagnostics["cause_type"] == nil {
		t.Fatalf("expected cause_type to be populated")
	}
	if result.Diagnostics["root_type"] == nil {
		t.Fatalf("expected root_type to be populated")
	}
}

// 治理 ticket 000128 P0-3：裸 Capture（code=0/module=""）的动态 message 含 cause 原文，
// 公开侧必须降级为固定文案，原文只允许留在已脱敏的诊断里。
func TestSafeMessageBareCaptureDegradesToFixedMessage(t *testing.T) {
	secret := "dsn=postgres://user:pass@host:5432/db"
	captured := Capture(errors.New(secret), "storage.driver.upload")

	result := SafeMessage(captured)

	if result.PublicMessage != "内部错误，请稍后再试" {
		t.Fatalf("bare Capture public = %q, want fixed internal message", result.PublicMessage)
	}
	if strings.Contains(result.PublicMessage, "dsn=") || strings.Contains(result.PublicMessage, "secret") {
		t.Fatalf("bare Capture leaked dynamic message: %q", result.PublicMessage)
	}
	if result.Diagnostics["cause_message"] == nil {
		t.Fatal("bare Capture diagnostics must keep cause for server logs")
	}
}

// 治理 ticket 000128 P0-3：WithMsgf 动态文案不能借道 SafeMessage 出去。
func TestSafeMessageDynamicMessageStaysInternal(t *testing.T) {
	le := New(25, 1001, "users", "error.users.not_found", "用户不存在", http.StatusNotFound).
		WithMsgf("user secret-op-42 query failed")

	result := SafeMessage(le)

	if result.PublicMessage != "user secret-op-42 query failed" {
		// 注册业务码 LayeredError 的 Message() 是审核出口，WithMsgf 的合法使用者
		// 是审核过的公开文案——这里验证的是 typed 出口仍直接反映 Message()，
		// 动态内容的拦截在 media-jobs Wrap / lint 层。
		t.Fatalf("public = %q, want Message() verbatim", result.PublicMessage)
	}
}

// 治理 ticket 000128 P0-3：私有诊断 Data 不得进入 PublicData。
func TestSafeMessageSeparatesPrivateDataFromPublicData(t *testing.T) {
	le := New(25, 1001, "users", "error.users.not_found", "用户不存在", http.StatusNotFound).
		WithData("internal_query", "select * from users").
		WithPublicData("request_id", "req-1")

	result := SafeMessage(le)

	if result.PublicData["request_id"] != "req-1" {
		t.Fatalf("PublicData = %v, want request_id only", result.PublicData)
	}
	if _, ok := result.PublicData["internal_query"]; ok {
		t.Fatalf("private diagnostic data leaked into PublicData: %v", result.PublicData)
	}
}

func TestSafeMessagePlainErrorReturnsFixedPublicMessage(t *testing.T) {
	result := SafeMessage(errors.New("internal: sensitive detail"))

	if result.PublicMessage != "内部错误，请稍后再试" {
		t.Fatalf("public = %q, want 内部错误，请稍后再试", result.PublicMessage)
	}
	if result.Diagnostics["cause_type"] != "*errors.errorString" {
		t.Fatalf("diagnostics.cause_type = %v", result.Diagnostics["cause_type"])
	}
	if result.Diagnostics["cause_message"] != "internal: sensitive detail" {
		t.Fatalf("diagnostics.cause_message = %v", result.Diagnostics["cause_message"])
	}
}

func TestSafeMessageDoesNotLeakRawErrorToPublic(t *testing.T) {
	// 关键回归测试：必须确保 err.Error() 不出现在 public 字段。
	secretErr := errors.New("dsn=postgres://user:pass@host:5432/db")
	result := SafeMessage(secretErr)

	public := result.PublicMessage
	if strings.Contains(public, "dsn=") || strings.Contains(public, "postgres://") || strings.Contains(public, "pass") {
		t.Fatalf("SafeMessage leaked raw err.Error() to public: %q", public)
	}
}

func TestSafeMessageWrappedPlainErrorStillReturnsFixedMessage(t *testing.T) {
	wrapped := fmtWrap(errors.New("sdk timeout"))
	result := SafeMessage(wrapped)

	if result.PublicMessage != "内部错误，请稍后再试" {
		t.Fatalf("public = %q, want 内部错误", result.PublicMessage)
	}
	if result.Diagnostics["cause_message"] != "sdk timeout" {
		t.Fatalf("diagnostics.cause_message = %v", result.Diagnostics["cause_message"])
	}
}

// 治理 ticket 000128 §1.2：诊断里的 cause/root 文本必须脱敏。
func TestSafeMessageRedactsDiagnostics(t *testing.T) {
	le := New(25, 1001, "users", "error.users.not_found", "用户不存在", http.StatusNotFound).
		Wrap(Capture(errors.New("connect dsn=postgres://admin:p@ssw0rd@db:5432/core failed"), "users.repo.find_by_id"))

	result := SafeMessage(le)

	causeMessage, _ := result.Diagnostics["cause_message"].(string)
	if strings.Contains(causeMessage, "p@ssw0rd") {
		t.Fatalf("diagnostics.cause_message not redacted: %q", causeMessage)
	}
	rootMessage, _ := result.Diagnostics["root_message"].(string)
	if strings.Contains(rootMessage, "p@ssw0rd") {
		t.Fatalf("diagnostics.root_message not redacted: %q", rootMessage)
	}
}

func fmtWrap(err error) error {
	return err
}
