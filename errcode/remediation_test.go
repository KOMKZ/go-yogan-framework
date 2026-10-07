package errcode

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// 治理 ticket 000395 独立 review 整改回归（P1-1 / P1-2 / P1-3A）。

// --- P1-1：链遍历必须支持任意合法 error 实现 ---

// uncomparableError 本身是 slice 且用值接收器实现 error；它装入 error
// 接口后的动态值真正不可比较，作为 map key 会 panic。
type uncomparableError []string

func (e uncomparableError) Error() string { return strings.Join(e, "|") }

// cycleError 自引用 Unwrap，构造自定义环。
type cycleError struct{ n int }

func (e *cycleError) Error() string { return "cycle" }
func (e *cycleError) Unwrap() error { return e }

// branchingCycleError 同时把自己暴露为两个 Join 分支。仅限制单分支深度
// 会指数展开；调用计数使回归在挂死前 fail loud。
type branchingCycleError struct{ unwrapCalls int }

func (e *branchingCycleError) Error() string { return "branching cycle" }
func (e *branchingCycleError) Unwrap() []error {
	e.unwrapCalls++
	if e.unwrapCalls > maxChainNodes {
		panic("error chain traversal exceeded the global node budget")
	}
	return []error{e, e}
}

func TestChainWalkSurvivesUncomparableError(t *testing.T) {
	cause := uncomparableError{"alpha", "beta"}
	captured := Capture(cause, "probe.uncomparable")
	diaged := CaptureWithDiag(cause, "probe.uncomparable", map[string]any{"asset_id": "asset_1"})

	for name, err := range map[string]error{"capture": captured, "captureWithDiag": diaged} {
		var le *LayeredError
		if !errors.As(err, &le) {
			t.Fatalf("%s: expected LayeredError, got %T", name, err)
		}
		if got := le.Operation(); got != "probe.uncomparable" {
			t.Fatalf("%s: operation = %q", name, got)
		}
		// 出口字段生成走同一条链遍历，不得 panic。
		core, logs := observer.New(zapcore.InfoLevel)
		zap.New(core).Error("failed", ErrorLogFields(err)...)
		if len(logs.All()) != 1 {
			t.Fatalf("%s: expected one log entry", name)
		}
	}
	if got := DiagnosticFields(diaged)["asset_id"]; got != "asset_1" {
		t.Fatalf("diagnostics on uncomparable chain = %v", got)
	}
}

func TestChainWalkBoundsTotalWorkAcrossBranchingCycle(t *testing.T) {
	cause := &branchingCycleError{}
	if layers := layeredErrorChain(cause); len(layers) != 0 {
		t.Fatalf("unexpected layered errors: %d", len(layers))
	}
	if cause.unwrapCalls > maxChainNodes {
		t.Fatalf("unwrap calls = %d, want <= %d", cause.unwrapCalls, maxChainNodes)
	}
}

func TestChainWalkTerminatesOnCustomUnwrapCycle(t *testing.T) {
	cause := &cycleError{n: 1}
	captured := Capture(cause, "probe.cycle")
	var le *LayeredError
	if !errors.As(captured, &le) {
		t.Fatalf("expected LayeredError, got %T", captured)
	}
	if got := le.Operation(); got != "probe.cycle" {
		t.Fatalf("operation = %q", got)
	}
	// 环在 maxChainDepth 截断：链文本仍可生成，出口字段不 panic、不挂死。
	core, logs := observer.New(zapcore.InfoLevel)
	zap.New(core).Error("failed", ErrorLogFields(captured)...)
	if len(logs.All()) != 1 {
		t.Fatalf("expected one log entry")
	}
}

// --- P1-2：直接 LayeredError 场景下“内层首发值优先” ---

func deepDiagError(marker string) *LayeredError {
	return Capture(errors.New("deep cause"), "deep.op").(*LayeredError).
		WithDiag("asset_id", marker)
}

func TestCaptureWithDiagDirectLayeredErrorKeepsInnerValue(t *testing.T) {
	deep := deepDiagError("asset_deep")
	outer := CaptureWithDiag(deep, "outer.op", map[string]any{"asset_id": "asset_outer"})

	if got := DiagnosticFields(outer)["asset_id"]; got != "asset_deep" {
		t.Fatalf("DiagnosticFields[asset_id] = %v, want innermost asset_deep", got)
	}
	// 首发层本体不得被改写。
	if got := DiagnosticFields(deep)["asset_id"]; got != "asset_deep" {
		t.Fatalf("deep layer data mutated: %v", got)
	}
	// 上层补缺失字段仍然可见。
	outer = CaptureWithDiag(deep, "outer.op", map[string]any{"slot_key": "slot_outer"})
	if got := DiagnosticFields(outer)["slot_key"]; got != "slot_outer" {
		t.Fatalf("missing-field supplement lost: %v", DiagnosticFields(outer))
	}
}

func TestCaptureWithDiagDirectLayeredErrorKeepsChainSemantics(t *testing.T) {
	sentinel := errors.New("deep cause")
	deep := Capture(sentinel, "deep.op").(*LayeredError).WithDiag("asset_id", "asset_deep")
	biz := New(19, 1001, "ccv3", "error.ccv3.wrap", "包装错误", http.StatusConflict).
		WithPublicData("retry_after_ms", 500).
		Wrap(deep)
	outer := CaptureWithDiag(biz, "outer.op", map[string]any{"asset_id": "asset_outer"})

	if !errors.Is(outer, sentinel) {
		t.Fatal("errors.Is lost deep sentinel through direct-layer wrapping")
	}
	if !errors.Is(outer, biz) {
		t.Fatal("errors.Is lost registered business code")
	}
	if got := DiagnosticFields(outer)["asset_id"]; got != "asset_deep" {
		t.Fatalf("asset_id = %v, want innermost", got)
	}
	if got := outermostRegisteredError(outer); got == nil || got.Code() != 191001 {
		t.Fatalf("registered code lookup = %+v", got)
	}
	public := SafeMessage(outer)
	if public.PublicMessage != "包装错误" || public.PublicData["retry_after_ms"] != 500 {
		t.Fatalf("public semantics changed: %+v", public)
	}
	// origin/operation 继承内层首发边界（与嵌套 Capture 语义一致）。
	var le *LayeredError
	if !errors.As(outer, &le) {
		t.Fatal("expected LayeredError")
	}
	if le.Code() != biz.Code() || le.Module() != biz.Module() || le.HTTPStatus() != http.StatusConflict {
		t.Fatalf("outer registered semantics = code:%d module:%q status:%d", le.Code(), le.Module(), le.HTTPStatus())
	}
	if !strings.Contains(le.OriginStack(), "TestCaptureWithDiagDirectLayeredErrorKeepsChainSemantics") {
		t.Fatalf("origin stack lost innermost site:\n%s", le.OriginStack())
	}
}

func TestCaptureWithDiagConflictAcrossWrapsAndJoin(t *testing.T) {
	deep := deepDiagError("asset_deep")
	// 一层/多层 %w：内层首发值优先。
	once := CaptureWithDiag(nestedChainWrapOnce(deep), "outer.op", map[string]any{"asset_id": "asset_outer"})
	if got := DiagnosticFields(once)["asset_id"]; got != "asset_deep" {
		t.Fatalf("one-level %%w: asset_id = %v", got)
	}
	twice := CaptureWithDiag(nestedChainWrapTwice(deep), "outer.op", map[string]any{"asset_id": "asset_outer"})
	if got := DiagnosticFields(twice)["asset_id"]; got != "asset_deep" {
		t.Fatalf("multi-level %%w: asset_id = %v", got)
	}
	// errors.Join：确定性取 DFS 末支首发值，外层字段不得覆盖任何分支。
	joined := CaptureWithDiag(
		fmt.Errorf("fanout: %w", errors.Join(deepDiagError("asset_a"), deepDiagError("asset_b"))),
		"outer.op", map[string]any{"asset_id": "asset_outer"})
	if got := DiagnosticFields(joined)["asset_id"]; got != "asset_b" {
		t.Fatalf("join: asset_id = %v, want deterministic last-branch asset_b", got)
	}
}

// --- P1-3A：允许键装不进原文（结构约束） ---

func TestDiagnosticTokenCharsetDropsRawText(t *testing.T) {
	base := Capture(errors.New("x"), "op").(*LayeredError)
	poisoned := base.WithDiagFields(map[string]any{
		"asset_id":            "快速旋转 带空格",
		"validation_actual":   "ERRCHAIN_PROBE_SENTINEL user prompt fragment",
		"provider_error_code": "invalid_request_error: input echoed back",
		"model_key":           "qwen-plus",
		"validation_rule":     "whitelist_enum",
	})
	fields := DiagnosticFields(poisoned)
	if _, ok := fields["asset_id"]; ok {
		t.Fatalf("raw CJK/space text must be dropped from asset_id: %+v", fields)
	}
	if _, ok := fields["validation_actual"]; ok {
		t.Fatalf("raw prompt fragment must be dropped from validation_actual: %+v", fields)
	}
	if _, ok := fields["provider_error_code"]; ok {
		t.Fatalf("message-style text must be dropped from provider_error_code: %+v", fields)
	}
	if got := fields["model_key"]; got != "qwen-plus" {
		t.Fatalf("legit token dropped: %v", got)
	}
	if got := fields["validation_rule"]; got != "whitelist_enum" {
		t.Fatalf("legit enum dropped: %v", got)
	}
	// 超长原文整值丢弃，不保留前缀。
	long := base.WithDiag("asset_id", strings.Repeat("a", 200))
	if _, ok := DiagnosticFields(long)["asset_id"]; ok {
		t.Fatal("over-length value must be dropped, not truncated")
	}
	// 出口 error_data 同样不含被丢弃键。
	core, logs := observer.New(zapcore.InfoLevel)
	zap.New(core).Error("failed", ErrorLogFields(poisoned)...)
	serialized := fmt.Sprint(observerFieldsMap(logs.All()[0]))
	if strings.Contains(serialized, "ERRCHAIN_PROBE_SENTINEL") || strings.Contains(serialized, "快速旋转") {
		t.Fatalf("raw text leaked into error_data: %s", serialized)
	}
}
