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

// 治理 ticket 000395 批次一：嵌套 %w 包装后的 LayeredError 保真与受控诊断字段。
//
// #394 故障注入结论：深层已 Capture 的 LayeredError 被中间 fmt.Errorf("%w") 包装后，
// Capture 只做直接类型断言，执行层二次 Capture 会把 origin/operation 覆盖为执行层位置，
// 且 ErrorLogFields 只读最外层 Data()，深层受控诊断字段全部不可见。
// 本文件用表驱动测试固定五类链路的期望行为：
// 普通 error / 直接 LayeredError / 一层 %w / 多层 %w / 业务码 Wrap，另加 CaptureInto defer
// 与 errors.Join 变体；诊断字段、负向脱敏和 errors.Is/As 兼容各自独立成测试。

// errNestedSentinel 深层技术错误的 typed 根因，用于 errors.As 兼容断言。
type errNestedSentinel struct{ msg string }

func (e *errNestedSentinel) Error() string { return e.msg }

// nestedChainDeepValidate 模拟深层校验边界：错误首次在此确认并 Capture。
// origin stack 首帧必须始终是本函数，不得被任何上层包装/二次捕获覆盖。
func nestedChainDeepValidate() *LayeredError {
	base := &errNestedSentinel{msg: "segment 2 duration_ms must be positive"}
	captured := Capture(base, "content_creation_v3.visual_plan.validate_segment").(*LayeredError)
	return captured.
		WithDiag("asset_id", "asset_1001").
		WithDiag("validation_field", "duration_ms").
		WithDiag("validation_rule", "required_positive").
		WithDiag("validation_actual", -3).
		WithDiag("raw_prompt", "必须被词表丢弃的原始 prompt 文本")
}

// nestedChainWrapOnce / nestedChainWrapTwice 模拟中间层普通 %w 包装（不经过 LayeredError）。
func nestedChainWrapOnce(deep error) error {
	return fmt.Errorf("llm execute stage: %w", deep)
}

func nestedChainWrapTwice(deep error) error {
	return fmt.Errorf("orchestrate: %w", nestedChainWrapOnce(deep))
}

// nestedChainExecutorRecapture 模拟执行层拿到"可能已被包装过"的错误后二次 Capture。
// 缺陷路径下 origin 首帧会变成这里；修复后深层 origin 必须保留。
func nestedChainExecutorRecapture(cause error, operation string) error {
	return Capture(cause, operation)
}

// nestedChainCaptureIntoDefer 模拟 service 入口 defer CaptureInto 的包装路径。
func nestedChainCaptureIntoDefer(deep error) (err error) {
	defer CaptureInto(&err, "content_creation_v3.stage_service.run")
	err = nestedChainWrapOnce(deep)
	if err != nil {
		return err
	}
	return nil
}

// nestedChainExpect 单条链路的期望断言集合。
type nestedChainExpect struct {
	name string
	// build 构造最终返回给出口的错误。
	build func() error
	// wantOriginFrom 必须出现在 origin stack 首帧的函数名。
	wantOriginFrom string
	// forbidOriginFrom 缺陷路径下才会出现的函数名；为空则不检查。
	forbidOriginFrom string
	// wantOperation 期望的 error_operation。
	wantOperation string
	// wantRegisteredCode>0 时断言日志出口暴露该注册业务码与文案。
	wantRegisteredCode int
	wantRegisteredMsg  string
	// checkChainCompatibility 时追加 errors.As/RootCause 深链兼容断言。
	checkChainCompatibility bool
}

func nestedChainCases() []nestedChainExpect {
	const executorOp = "content_creation_v3.llm_text_executor.run"
	deepOp := "content_creation_v3.visual_plan.validate_segment"
	bizTemplate := New(19, 1001, "ccv3", "error.ccv3.output_invalid", "生成结果不合法", http.StatusInternalServerError)
	return []nestedChainExpect{
		{
			name:           "普通 error 直接捕获：origin 即捕获位置",
			build:          func() error { return nestedChainExecutorRecapture(errors.New("boom"), executorOp) },
			wantOriginFrom: "nestedChainExecutorRecapture",
			wantOperation:  executorOp,
		},
		{
			name:           "直接 LayeredError：Capture 原样返回不重包",
			build:          func() error { return nestedChainExecutorRecapture(nestedChainDeepValidate(), executorOp) },
			wantOriginFrom: "nestedChainDeepValidate",
			wantOperation:  deepOp,
		},
		{
			name: "一层 %w 包装后二次捕获：保留深层 origin/operation",
			build: func() error {
				return nestedChainExecutorRecapture(nestedChainWrapOnce(nestedChainDeepValidate()), executorOp)
			},
			wantOriginFrom:          "nestedChainDeepValidate",
			forbidOriginFrom:        "nestedChainExecutorRecapture",
			wantOperation:           deepOp,
			checkChainCompatibility: true,
		},
		{
			name: "多层 %w 包装后二次捕获：仍保留最内层 origin",
			build: func() error {
				return nestedChainExecutorRecapture(nestedChainWrapTwice(nestedChainDeepValidate()), executorOp)
			},
			wantOriginFrom:          "nestedChainDeepValidate",
			forbidOriginFrom:        "nestedChainExecutorRecapture",
			wantOperation:           deepOp,
			checkChainCompatibility: true,
		},
		{
			name: "业务码 Wrap 后再 %w：origin 保留且注册语义可还原",
			build: func() error {
				wrapped := bizTemplate.Wrap(nestedChainDeepValidate())
				return nestedChainExecutorRecapture(nestedChainWrapOnce(wrapped), executorOp)
			},
			wantOriginFrom:          "nestedChainDeepValidate",
			forbidOriginFrom:        "nestedChainExecutorRecapture",
			wantOperation:           deepOp,
			wantRegisteredCode:      191001,
			wantRegisteredMsg:       "生成结果不合法",
			checkChainCompatibility: true,
		},
		{
			name: "CaptureInto defer 遇嵌套 %w：不覆盖深层 origin/operation",
			build: func() error {
				err := nestedChainCaptureIntoDefer(nestedChainDeepValidate())
				if err == nil {
					err = errors.New("expected wrapped error")
				}
				return err
			},
			wantOriginFrom:          "nestedChainDeepValidate",
			forbidOriginFrom:        "nestedChainCaptureIntoDefer",
			wantOperation:           deepOp,
			checkChainCompatibility: true,
		},
	}
}

func TestNestedChainOriginFidelity(t *testing.T) {
	for _, tc := range nestedChainCases() {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.build()
			if err == nil {
				t.Fatal("expected non-nil error")
			}
			var le *LayeredError
			if !errors.As(err, &le) {
				t.Fatalf("expected *LayeredError via errors.As, got %T", err)
			}
			origin := le.OriginStack()
			if !strings.Contains(origin, tc.wantOriginFrom) {
				t.Fatalf("origin stack lost innermost site %q, got:\n%s", tc.wantOriginFrom, origin)
			}
			if tc.forbidOriginFrom != "" && strings.Contains(origin, tc.forbidOriginFrom) {
				t.Fatalf("origin stack was overwritten by outer capture %q, got:\n%s", tc.forbidOriginFrom, origin)
			}
			if got := le.Operation(); got != tc.wantOperation {
				t.Fatalf("operation = %q, want %q", got, tc.wantOperation)
			}
			if tc.wantRegisteredCode > 0 {
				assertNestedRegisteredCodeVisible(t, err, tc.wantRegisteredCode, tc.wantRegisteredMsg)
			}
			if !tc.checkChainCompatibility {
				return
			}
			// errors.As：深层 typed 根因必须仍可匹配。
			var sentinel *errNestedSentinel
			if !errors.As(err, &sentinel) {
				t.Fatal("errors.As lost the deep typed cause")
			}
			// RootCause：仍指向链上最深处。
			if _, ok := le.RootCause().(*errNestedSentinel); !ok {
				t.Fatalf("RootCause() = %T, want *errNestedSentinel", le.RootCause())
			}
		})
	}
}

// assertNestedRegisteredCodeVisible 断言出口日志暴露链上注册业务码与注册文案。
func assertNestedRegisteredCodeVisible(t *testing.T, err error, code int, msg string) {
	t.Helper()
	core, logs := observer.New(zapcore.InfoLevel)
	zap.New(core).Error("queue task failed", ErrorLogFields(err)...)
	fields := observerFieldsMap(logs.All()[0])
	if got := fields["error_code"]; got != int64(code) {
		t.Fatalf("error_code = %v, want %d", got, code)
	}
	if got := fields["error_msg"]; got != msg {
		t.Fatalf("error_msg = %v, want %q", got, msg)
	}
}

func TestNestedChainPreservesBusinessCodeSemantics(t *testing.T) {
	bizTemplate := New(19, 1001, "ccv3", "error.ccv3.output_invalid", "生成结果不合法", http.StatusInternalServerError).
		WithPublicData("stage", "llm_output")
	wrapped := nestedChainExecutorRecapture(nestedChainWrapOnce(bizTemplate.Wrap(nestedChainDeepValidate())), "executor.run")

	if !errors.Is(wrapped, bizTemplate) {
		t.Fatal("errors.Is lost the registered business code through nested wrapping")
	}
	var le *LayeredError
	if !errors.As(wrapped, &le) {
		t.Fatal("errors.As lost the outermost LayeredError")
	}
	if got := PublicMessageOf(wrapped); got != "内部错误，请稍后再试" {
		t.Fatalf("PublicMessageOf(nested capture) = %q, want fixed internal message (public side unchanged)", got)
	}
	if got := le.RootCause(); got == nil || got.Error() != "segment 2 duration_ms must be positive" {
		t.Fatalf("RootCause() = %v, want deep sentinel message", got)
	}
	if cause := le.DiagnosticCause(); cause == nil || !strings.Contains(cause.Error(), "llm execute stage") {
		t.Fatalf("DiagnosticCause() = %v, want contextual wrap layer", cause)
	}
	// SafeMessage public 侧不得因嵌套捕获变化。
	if safe := SafeMessage(wrapped); safe.PublicMessage != "内部错误，请稍后再试" {
		t.Fatalf("SafeMessage public message = %q", safe.PublicMessage)
	}
}

func TestErrorLogFieldsNestedChainSurfacesDeepFacts(t *testing.T) {
	bizTemplate := New(19, 1001, "ccv3", "error.ccv3.output_invalid", "生成结果不合法", http.StatusInternalServerError)
	wrapped := nestedChainExecutorRecapture(
		nestedChainWrapOnce(bizTemplate.Wrap(nestedChainDeepValidate())),
		"content_creation_v3.llm_text_executor.run",
	)

	core, logs := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)
	logger.Error("queue task failed", ErrorLogFields(wrapped)...)

	fields := observerFieldsMap(logs.All()[0])

	// 外层为裸 Capture（code==0）时，链上注册业务码必须补进出口字段。
	if got := fields["error_code"]; got != int64(191001) {
		t.Fatalf("error_code = %v, want innermost registered 191001", got)
	}
	if got := fields["error_msg"]; got != "生成结果不合法" {
		t.Fatalf("error_msg = %v, want registered message", got)
	}
	if got := fields["error_operation"]; got != "content_creation_v3.visual_plan.validate_segment" {
		t.Fatalf("error_operation = %v, want deep operation", got)
	}
	if origin, _ := fields["error_origin_stack"].(string); !strings.Contains(origin, "nestedChainDeepValidate") {
		t.Fatalf("error_origin_stack lost deep site: %q", origin)
	}

	// error_data 必须包含深层受控诊断字段（词表内、安全标量）。
	data, _ := fields["error_data"].(map[string]any)
	if data == nil {
		t.Fatalf("error_data missing from log fields: %+v", fields)
	}
	for key, want := range map[string]any{
		"asset_id":          "asset_1001",
		"validation_field":  "duration_ms",
		"validation_rule":   "required_positive",
		"validation_actual": int64(-3),
	} {
		got, ok := data[key]
		if !ok {
			t.Fatalf("error_data missing deep diagnostic %q: %+v", key, data)
		}
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Fatalf("error_data[%q] = %v, want %v", key, got, want)
		}
	}
	// 词表外键（含原始 prompt）不得随链传播进日志。
	for _, forbidden := range []string{"raw_prompt", "prompt", "model_output"} {
		if _, ok := data[forbidden]; ok {
			t.Fatalf("error_data leaked non-vocabulary key %q: %+v", forbidden, data)
		}
	}
}

func TestErrorLogFieldsDiagnosticPrecedenceAndSafety(t *testing.T) {
	deep := Capture(errors.New("provider failed"), "provider.chat.complete").(*LayeredError).
		WithDiag("asset_id", "asset_deep").
		WithDiag("provider_error_code", "token=eyJhbGciOiJIUzI1NiJ9.secret signature=abc123def")
	// 外层各自字段：非词表键保留原样；词表键被深层首发值覆盖（首发确认边界最可信）。
	outer := Capture(nestedChainWrapOnce(deep), "stage.execute").(*LayeredError).
		WithData("custom_outer", "keep-me").
		WithData("asset_id", "asset_outer_should_lose")

	fieldsMap := func(err error) map[string]any {
		core, logs := observer.New(zapcore.InfoLevel)
		zap.New(core).Error("failed", ErrorLogFields(err)...)
		return observerFieldsMap(logs.All()[0])
	}

	fields := fieldsMap(outer)
	data, _ := fields["error_data"].(map[string]any)
	if data["custom_outer"] != "keep-me" {
		t.Fatalf("outer non-vocabulary data lost: %+v", data)
	}
	if data["asset_id"] != "asset_deep" {
		t.Fatalf("error_data[asset_id] = %v, want innermost value", data["asset_id"])
	}
	// 整改 P1-3A：含空格/引号/= 的原文形态不符合安全 token 字符集，整值丢弃——
	// 任何凭据片段（token=/signature= 载体）在允许键中都不可能存活。
	if _, ok := data["provider_error_code"]; ok {
		t.Fatalf("non-token provider_error_code must be dropped entirely: %+v", data)
	}
	if clean := Capture(errors.New("x"), "op").(*LayeredError).
		WithDiag("provider_error_code", "invalid_request_error"); DiagnosticFields(clean)["provider_error_code"] != "invalid_request_error" {
		t.Fatalf("legit provider_error_code dropped: %+v", DiagnosticFields(clean))
	}

	// 安全标量约束：非标量值丢弃；超长/含空格原文整值丢弃；未知键丢弃。
	weird := Capture(errors.New("bad"), "op").(*LayeredError).
		WithDiag("validation_actual", map[string]any{"deep": "object"}).
		WithDiag("validation_actual", strings.Repeat("x", 500)).
		WithDiag("assetId", "typo-key-must-drop")
	data, _ = fieldsMap(weird)["error_data"].(map[string]any)
	if _, ok := data["assetId"]; ok {
		t.Fatalf("unknown diagnostic key must be dropped: %+v", data)
	}
	if _, ok := data["validation_actual"]; ok {
		t.Fatalf("over-length validation_actual must be dropped (not truncated): %+v", data)
	}
}

func TestErrorLogFieldsNestedPlainChainStaysClean(t *testing.T) {
	// 普通错误链（无任何 LayeredError）行为不变：只有 error_chain。
	plain := fmt.Errorf("outer: %w", errors.New("boom with token=abcdef.123456"))
	core, logs := observer.New(zapcore.InfoLevel)
	zap.New(core).Error("failed", ErrorLogFields(plain)...)
	fields := observerFieldsMap(logs.All()[0])
	if _, ok := fields["error_code"]; ok {
		t.Fatalf("plain chain must not carry error_code: %+v", fields)
	}
	chain, _ := fields["error_chain"].(string)
	if strings.Contains(chain, "abcdef.123456") {
		t.Fatalf("plain chain leaked token: %q", chain)
	}
}

func TestNestedChainJoinedErrorsKeepDeterministicOrigin(t *testing.T) {
	deepA := Capture(errors.New("branch a failed"), "branch.a")
	deepB := Capture(errors.New("branch b failed"), "branch.b")
	joined := nestedChainExecutorRecapture(
		fmt.Errorf("fanout: %w", errors.Join(deepA, deepB)),
		"fanout.run",
	)
	var le *LayeredError
	if !errors.As(joined, &le) {
		t.Fatal("expected *LayeredError")
	}
	// Join 的 origin 取 DFS 最内层分支（确定性规则），两个分支的原文仍在 error_chain 可见。
	if origin := le.OriginStack(); !strings.Contains(origin, "TestNestedChainJoinedErrorsKeepDeterministicOrigin") &&
		!strings.Contains(origin, "branch") {
		t.Fatalf("joined chain origin missing branch site: %q", origin)
	}
	if got := le.Operation(); got != "branch.b" {
		t.Fatalf("joined chain operation = %q, want deterministic innermost branch.b", got)
	}
	if !strings.Contains(joined.Error(), "branch a failed") || !strings.Contains(joined.Error(), "branch b failed") {
		t.Fatalf("joined chain text lost a branch: %q", joined.Error())
	}
}

func TestDiagnosticFieldsVocabularyIsFrozen(t *testing.T) {
	keys := DiagnosticKeys()
	if len(keys) == 0 {
		t.Fatal("diagnostic vocabulary must not be empty")
	}
	for i := 1; i < len(keys); i++ {
		if keys[i-1] >= keys[i] {
			t.Fatalf("DiagnosticKeys must be sorted & unique: %v", keys)
		}
	}
	for _, required := range []string{
		"creation_id", "stage_id", "asset_id", "slot_key",
		"validation_field", "validation_rule", "validation_actual",
		"provider_key", "model_key", "provider_request_id", "provider_error_code",
		"storage_id", "segment_index", "input_sha256", "output_bytes", "item_count",
	} {
		if !IsDiagnosticKey(required) {
			t.Fatalf("frozen vocabulary missing %q", required)
		}
	}
}
