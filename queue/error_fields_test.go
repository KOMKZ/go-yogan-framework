package queue

import (
	"errors"
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

	if fields["error"] == "" || !strings.Contains(fields["error_chain"].(string), "任务执行失败") {
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

	if fields["error"] != "plain failure" || fields["error_chain"] != "plain failure" {
		t.Fatalf("plain error fields = %+v", fields)
	}
	if _, ok := fields["error_origin_stack"]; ok {
		t.Fatalf("plain error should not have origin stack: %+v", fields)
	}
}

func zapFieldsMap(fields []zap.Field) map[string]interface{} {
	encoder := zapcore.NewMapObjectEncoder()
	for _, field := range fields {
		field.AddTo(encoder)
	}
	return encoder.Fields
}
