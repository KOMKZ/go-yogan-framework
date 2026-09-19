package errcode

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// 治理 ticket 000128 P1-1：用 zap observer 拿真实日志条目做端到端断言，
// 验证字段值与脱敏效果，而不是只测字段名数组。
func TestErrorLogFieldsObserverEndToEnd(t *testing.T) {
	core, logs := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)

	secret := "dsn=postgres://admin:p@ssw0rd@db:5432/core token=eyJhbGciOiJIUzI1NiJ9.secret"
	captured := Capture(errors.New(secret), "storage.driver.upload")
	layered := New(27, 1002, "storage", "error.storage.upload_failed", "上传失败", http.StatusInternalServerError).Wrap(captured)

	logger.Error("upload failed", ErrorLogFields(layered)...)

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	entry := entries[0]
	fields := observerFieldsMap(entry)

	if fields["error_code"] != int64(271002) {
		t.Fatalf("error_code = %v, want 271002", fields["error_code"])
	}
	if fields["error_msg"] != "上传失败" {
		t.Fatalf("error_msg = %v, want registered message", fields["error_msg"])
	}
	if fields["error_operation"] != "storage.driver.upload" {
		t.Fatalf("error_operation = %v", fields["error_operation"])
	}
	for _, key := range []string{"error_chain", "error_cause_message", "error_root_message"} {
		value, _ := fields[key].(string)
		if strings.Contains(value, "p@ssw0rd") || strings.Contains(value, "eyJhbGciOiJIUzI1NiJ9") {
			t.Fatalf("%s leaked secret: %q", key, value)
		}
		if value == "" {
			t.Fatalf("%s must be present (redacted) in log entry", key)
		}
	}
	if fields["error_cause_type"] == nil || fields["error_root_type"] == nil {
		t.Fatalf("cause/root types must be present: %+v", fields)
	}
}

func TestErrorLogFieldsObserverPlainError(t *testing.T) {
	core, logs := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)

	logger.Error("plain failed", ErrorLogFields(errors.New("boom with token=abcdef.123456"))...)

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	fields := observerFieldsMap(entries[0])
	chain, _ := fields["error_chain"].(string)
	if strings.Contains(chain, "token=abcdef.123456") {
		t.Fatalf("plain error chain leaked token: %q", chain)
	}
	if _, ok := fields["error_code"]; ok {
		t.Fatalf("plain error must not carry error_code: %+v", fields)
	}
}

func TestErrorLogFieldsNil(t *testing.T) {
	if got := ErrorLogFields(nil); got != nil {
		t.Fatalf("nil err should produce nil fields, got %v", got)
	}
}

// PublicMessageOf 守卫：注册码给注册文案，裸 Capture 降级固定文案。
func TestPublicMessageOf(t *testing.T) {
	registered := New(27, 1003, "storage", "error.storage.not_found", "资源不存在", http.StatusNotFound)
	if got := PublicMessageOf(registered); got != "资源不存在" {
		t.Fatalf("PublicMessageOf(registered) = %q", got)
	}
	bare := Capture(errors.New("internal secret text"), "op")
	if got := PublicMessageOf(bare); got != "内部错误，请稍后再试" {
		t.Fatalf("PublicMessageOf(bare) = %q, want fixed message", got)
	}
}

func observerFieldsMap(entry observer.LoggedEntry) map[string]any {
	fields := map[string]any{}
	for _, field := range entry.Context {
		encoder := zapcore.NewMapObjectEncoder()
		field.AddTo(encoder)
		for k, v := range encoder.Fields {
			fields[k] = v
		}
	}
	return fields
}
