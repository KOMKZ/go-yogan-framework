package queue

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/KOMKZ/go-yogan-framework/errcode"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestQueueErrorFieldsIncludeLayeredDiagnostics(t *testing.T) {
	technical := errcode.Capture(errors.New("connection refused"), "queue.handler.execute")
	err := errcode.New(91, 7, "queue-test", "queue.test.failed", "任务执行失败", http.StatusInternalServerError).Wrap(technical)

	fields := zapFieldsMap(queueErrorFields(err))

	// 治理 ticket 000128 P1-1：queue 与 httpx/CLI 共用 errcode.ErrorLogFields，
	// 裸 zap.Error 字段已收掉，chain 走统一脱敏。
	if !strings.Contains(fields["error_chain"].(string), "任务执行失败") {
		t.Fatalf("missing error fields: %+v", fields)
	}
	if fields["error_code"] != int64(910007) || fields["error_msg"] != "任务执行失败" {
		t.Fatalf("unexpected layered fields: %+v", fields)
	}
	if fields["error_operation"] != "queue.handler.execute" {
		t.Fatalf("operation = %+v", fields["error_operation"])
	}
	if !strings.Contains(fields["error_origin_stack"].(string), "error_fields_test.go") {
		t.Fatalf("origin stack = %q", fields["error_origin_stack"])
	}
	if fields["error_cause_type"] == "" || fields["error_root_message"] != "connection refused" {
		t.Fatalf("missing cause fields: %+v", fields)
	}
}

func TestQueueErrorFieldsKeepPlainError(t *testing.T) {
	err := errors.New("plain failure")

	fields := zapFieldsMap(queueErrorFields(err))

	// 000128 P1-1：普通 error 只保留脱敏后的 error_chain，不再有原文 error 字段。
	if fields["error_chain"] != "plain failure" {
		t.Fatalf("plain error fields = %+v", fields)
	}
	if _, ok := fields["error"]; ok {
		t.Fatalf("raw error field must not be emitted (redacted chain only): %+v", fields)
	}
	if _, ok := fields["error_origin_stack"]; ok {
		t.Fatalf("plain error should not have origin stack: %+v", fields)
	}
}

// review 整改 A：queue 出口同样必须脱敏含 @ 的 DSN 密码，任何片段不得残留。
func TestQueueErrorFieldsRedactSecrets(t *testing.T) {
	technical := errcode.Capture(errors.New("connect dsn=postgres://admin:p@ssw0rd@db:5432/core failed"), "queue.handler.db")
	err := errcode.New(91, 8, "queue-test", "queue.test.db", "任务数据库错误", http.StatusInternalServerError).Wrap(technical)

	fields := zapFieldsMap(queueErrorFields(err))

	for _, key := range []string{"error_chain", "error_cause_message", "error_root_message"} {
		value, _ := fields[key].(string)
		if strings.Contains(value, "ssw0rd") || strings.Contains(value, "p@ss") {
			t.Fatalf("%s leaked dsn password fragment: %q", key, value)
		}
		if value == "" {
			t.Fatalf("%s must be present (redacted)", key)
		}
	}
}

func zapFieldsMap(fields []zap.Field) map[string]interface{} {
	encoder := zapcore.NewMapObjectEncoder()
	for _, field := range fields {
		field.AddTo(encoder)
	}
	return encoder.Fields
}

// queueNestedDeepCapture 模拟深层业务边界：错误首次在此确认并携带受控诊断。
// origin 首帧必须固定在本函数；queue 出口不得把它覆盖为消费层位置。
func queueNestedDeepCapture() error {
	return errcode.Capture(errors.New("segment 3 missing narration track"), "content_creation_v3.stage.validate").(*errcode.LayeredError).
		WithDiag("creation_id", "cr_test_0001").
		WithDiag("asset_id", "asset_3003").
		WithDiag("validation_field", "narration_track").
		WithDiag("validation_rule", "required_non_empty")
}

// 治理 ticket 000395 批次一消费方回归：hrise-worker 的真实出口是
// handleTask 返回 err → asynq ErrorHandler → queueErrorFields（asynq_server.go）。
// 深层错误经中间层 %w 包装、执行层二次 Capture 后，queue 单条 ERROR 仍必须能
// 定位深层源位置、operation 与受控诊断字段，且词表外内容不出现。
func TestQueueErrorFieldsPreserveNestedChainOrigin(t *testing.T) {
	deep := queueNestedDeepCapture()
	executorErr := errcode.Capture(fmt.Errorf("llm execute stage: %w", deep), "content_creation_v3.executor.run")
	taskErr := errcode.New(91, 7, "queue-test", "queue.test.failed", "任务执行失败", http.StatusInternalServerError).Wrap(executorErr)

	fields := zapFieldsMap(queueErrorFields(taskErr))

	if origin, _ := fields["error_origin_stack"].(string); !strings.Contains(origin, "queueNestedDeepCapture") {
		t.Fatalf("queue origin stack lost deep site: %q", origin)
	}
	if fields["error_operation"] != "content_creation_v3.stage.validate" {
		t.Fatalf("operation = %+v, want deep operation", fields["error_operation"])
	}
	if fields["error_code"] != int64(910007) || fields["error_msg"] != "任务执行失败" {
		t.Fatalf("outer registered code must win: %+v", fields)
	}
	data, _ := fields["error_data"].(map[string]interface{})
	for key, want := range map[string]string{
		"creation_id":      "cr_test_0001",
		"asset_id":         "asset_3003",
		"validation_field": "narration_track",
		"validation_rule":  "required_non_empty",
	} {
		if data[key] != want {
			t.Fatalf("error_data[%q] = %+v, want %q", key, data[key], want)
		}
	}
	if _, ok := data["prompt"]; ok {
		t.Fatalf("non-vocabulary key must not propagate: %+v", data)
	}
}
