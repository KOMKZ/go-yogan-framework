// log_schema_test.go 验证治理方案 ticket 000105 的 11 字段 schema 在 HTTP 错误日志中可见。
//
// 测试策略：
//   - 单元测试覆盖 requestContextFields 从 gin ctx 抽 3 个字段（request_id / user_id / client_ip）。
//   - 静态守卫：mustHaveFields 数组必须在增减时同步更新治理方案 §3.3。
//   - 集成验证在 lint 阶段以 rg 静态检查字段名出现位置保证不退化。

package httpx

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// 治理方案 ticket 000105 的 must-have 11 字段。
// 增/减字段必须同步更新 governance plan §3.3 和 §7 验证清单。
var mustHaveFields = []string{
	"error_code",
	"error_msg",
	"error_operation",
	"error_origin_stack",
	"error_cause_type",
	"error_cause_message",
	"error_root_type",
	"error_root_message",
	"request_id",
	"user_id",
	"client_ip",
}

// TestMustHaveSchemaDocumented 静态守卫：mustHaveFields 数量变化必须显式确认。
func TestMustHaveSchemaDocumented(t *testing.T) {
	expectedLen := 11
	if len(mustHaveFields) != expectedLen {
		t.Fatalf("mustHaveFields count changed: got %d, want %d (update governance plan if intentional)",
			len(mustHaveFields), expectedLen)
	}
}

// TestRequestContextFieldsExtractsRequestIDUserIDClientIP 验证 ctx 中 3 字段抽取正确。
func TestRequestContextFieldsExtractsRequestIDUserIDClientIP(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/test", nil)
	c.Set("trace_id", "trace-abc-123")
	c.Set("user_id", uint(42))

	fields := requestContextFields(c)
	names := fieldNames(fields)

	want := []string{"request_id", "user_id", "client_ip"}
	for _, w := range want {
		if !containsString(names, w) {
			t.Errorf("requestContextFields missing %q, got %v", w, names)
		}
	}
}

// TestRequestContextFieldsGracefullyHandlesMissing 验证空 ctx 不 panic。
func TestRequestContextFieldsGracefullyHandlesMissing(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/test", nil)

	fields := requestContextFields(c)
	for _, f := range fields {
		if f.Key == "" {
			t.Fatalf("empty key in field")
		}
	}
}

// TestRequestContextFieldsNilContext 验证 nil ctx 安全返回 nil。
func TestRequestContextFieldsNilContext(t *testing.T) {
	fields := requestContextFields(nil)
	if fields != nil {
		t.Fatalf("nil ctx should return nil, got %v", fields)
	}
}

// TestRequestContextFieldsOnlyEmitsWhatIsSet 验证缺失字段不写入空值。
func TestRequestContextFieldsOnlyEmitsWhatIsSet(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/test", nil)
	// 不设置 trace_id / user_id

	fields := requestContextFields(c)
	for _, f := range fields {
		if f.Key == "request_id" || f.Key == "user_id" {
			t.Errorf("field %q should not be emitted when ctx value is empty", f.Key)
		}
	}
}

// TestRequestContextFieldsTraceIDFromRequestContext 验证 trace_id 也可来自 request.Context()。
func TestRequestContextFieldsTraceIDFromRequestContext(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/test", nil)
	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), "trace_id", "trace-from-req-ctx"))

	fields := requestContextFields(c)
	names := fieldNames(fields)
	if !containsString(names, "request_id") {
		t.Fatalf("expected request_id from request.Context(), got %v", names)
	}
}

// TestRequestContextFieldsUserIDAcceptsAnyType 验证 user_id 类型不限（uint / string / int64）。
func TestRequestContextFieldsUserIDAcceptsAnyType(t *testing.T) {
	for _, v := range []any{uint(42), "uuid-123", int64(99), 100} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("GET", "/test", nil)
		c.Set("user_id", v)

		fields := requestContextFields(c)
		if len(fields) == 0 || fields[0].Key != "user_id" {
			t.Errorf("user_id field missing for value %v (%T)", v, v)
		}
	}
}

// helpers

func fieldNames(fields []zap.Field) []string {
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		out = append(out, f.Key)
	}
	return out
}

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
