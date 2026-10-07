// diag.go 冻结受控诊断字段词表与安全标量约束（治理 ticket 000395 批次一）。
//
// #394 结论：深层关键字段被上层 %w 包装后不会自动变成可见诊断信息，而把任意内层
// error_data 递归合并到最外层又有泄密风险。折中方案是"词表 + 安全标量"的受控传播：
//   - 只有词表内的键允许跨层进入统一日志的 error_data；
//   - 值必须是安全标量（字符串脱敏并截断为短摘要、布尔、数值）；
//   - 词表外键与非安全值在写入侧（WithDiag）和传播侧（DiagnosticFields）双端丢弃。
//
// 词表是冻结契约：新增键必须修改本文件并说明类别，禁止各业务自造同义字段。
package errcode

import (
	"reflect"
	"regexp"
)

// diagnosticKeyVocabulary 受控诊断字段冻结词表（实施方案 §5）。
// 类别：任务关联 / 合同校验 / 模型与 Provider / 媒体 / 输入输出摘要。
var diagnosticKeyVocabulary = map[string]struct{}{
	// 任务关联：只记稳定 ID，不记业务正文。
	"creation_id": {},
	"stage_id":    {},
	"asset_id":    {},
	"job_id":      {},
	"task_id":     {},
	"slot_key":    {},
	// 合同校验：validation_actual 只允许数值、布尔、枚举或短安全摘要。
	"output_contract":   {},
	"schema_version":    {},
	"validation_field":  {},
	"validation_rule":   {},
	"validation_actual": {},
	// 模型与 Provider：禁止 token、header、原始 body。
	"provider_key":        {},
	"model_key":           {},
	"provider_request_id": {},
	"provider_error_code": {},
	// 模型请求参数（000395 批次二补充）：LLM 复现诊断需要的安全数值事实。
	"temperature": {},
	"max_tokens":  {},
	// 媒体：URL 必须脱敏，优先记 storage_id。
	"storage_id":    {},
	"content_type":  {},
	"orientation":   {},
	"duration_ms":   {},
	"segment_index": {},
	// 输入/输出摘要：禁止原始文本或二进制内容。
	"input_bytes":   {},
	"output_bytes":  {},
	"input_sha256":  {},
	"output_sha256": {},
	"item_count":    {},
}

// maxDiagnosticValueRunes 受控诊断 string 值的最大长度（整改 P1-3A：由 200 收紧为
// 128 并配合字符集约束——截断不等于安全，超限原文直接丢弃而不是保留前缀）。
const maxDiagnosticValueRunes = 128

// diagnosticTokenPattern 受控诊断 string 值的唯一合法形态：ASCII 安全 token
// （字母/数字/._:@/+~-），长度 ≤128。空格、中文、引号、=、控制字符等意味着
// prompt/模型输出/用户正文/provider body 片段——一律整值丢弃，不做截断保留。
// 这是结构约束：允许键也装不进原文（治理 ticket 000395 整改 P1-3A）。
var diagnosticTokenPattern = regexp.MustCompile(`^[A-Za-z0-9._:@/+~-]{1,128}$`)

// IsDiagnosticKey 报告 key 是否在冻结诊断词表内。
func IsDiagnosticKey(key string) bool {
	_, ok := diagnosticKeyVocabulary[key]
	return ok
}

// DiagnosticKeys 返回排序后的词表全量键，供文档、工具与测试冻结 schema。
func DiagnosticKeys() []string {
	keys := make([]string, 0, len(diagnosticKeyVocabulary))
	for key := range diagnosticKeyVocabulary {
		keys = append(keys, key)
	}
	// 简单插入排序即可：词表规模固定且小。
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j-1] > keys[j]; j-- {
			keys[j-1], keys[j] = keys[j], keys[j-1]
		}
	}
	return keys
}

// WithDiag 添加受控诊断字段（返回新实例）。
// key 必须在冻结词表内、value 必须是安全标量，否则该字段被丢弃：
// 冻结词表是治理约束，词表外键不得进入统一日志 schema，也不得在业务层自造同义字段。
func (e *LayeredError) WithDiag(key string, value any) *LayeredError {
	if !IsDiagnosticKey(key) {
		return e
	}
	safe, ok := safeDiagnosticValue(value)
	if !ok {
		return e
	}
	clone := *e
	clone.data = e.cloneData()
	clone.data[key] = safe
	return &clone
}

// WithDiagFields 批量添加受控诊断字段（返回新实例），约束同 WithDiag。
func (e *LayeredError) WithDiagFields(fields map[string]any) *LayeredError {
	clone := *e
	clone.data = e.cloneData()
	for key, value := range fields {
		if !IsDiagnosticKey(key) {
			continue
		}
		if safe, ok := safeDiagnosticValue(value); ok {
			clone.data[key] = safe
		}
	}
	return &clone
}

// CaptureWithDiag 等价于 Capture 后一次性追加受控诊断字段，供"问题首次确认边界"
// 在无法对 error 返回值链式调用方法时使用。nil 语义与 Capture 一致。
//
// 整改 P1-2：cause 直接是 *LayeredError 时，Capture 会原样返回本体（身份语义），
// 此时诊断字段必须挂到"新的外层实例"而不是写进本体——否则上层同名键会覆盖
// 内层首发事实。外层继承内层 origin；operation 与嵌套 Capture 语义一致
// （内层已标注 op 时内层优先）。出口合并仍由 DiagnosticFields 保证内层优先。
func CaptureWithDiag(cause error, operation string, fields map[string]any) error {
	if cause == nil {
		return nil
	}
	if inner, isDirect := cause.(*LayeredError); isDirect {
		wrapper := &LayeredError{
			module:      inner.module,
			code:        inner.code,
			msgKey:      inner.msgKey,
			msg:         inner.msg,
			httpStatus:  inner.httpStatus,
			publicData:  inner.clonePublicData(),
			cause:       inner,
			operation:   operation,
			data:        make(map[string]interface{}),
			transparent: true,
		}
		if inner.OriginStack() != "" {
			wrapper.originStack = inner.OriginStack()
		} else {
			wrapper.originStack = errorOriginStack(cause)
		}
		if inner.Operation() != "" {
			wrapper.operation = inner.Operation()
		}
		return wrapper.WithDiagFields(fields)
	}
	captured := Capture(cause, operation)
	if captured == nil {
		return nil
	}
	layered, ok := captured.(*LayeredError)
	if !ok {
		return captured
	}
	return layered.WithDiagFields(fields)
}

// DiagnosticFields 汇总错误链上全部 LayeredError 的词表内安全诊断字段。
// 键冲突时内层（首发确认边界）覆盖外层：上层只补业务语义，不覆盖首发事实。
// 返回值只含已脱敏/已截断的安全标量，允许直接写入统一日志的 error_data。
func DiagnosticFields(err error) map[string]any {
	layers := layeredErrorChain(err)
	if len(layers) == 0 {
		return nil
	}
	merged := make(map[string]any, len(layers)*2)
	for _, layer := range layers {
		for key, value := range layer.Data() {
			if !IsDiagnosticKey(key) {
				continue
			}
			if safe, ok := safeDiagnosticValue(value); ok {
				merged[key] = safe
			}
		}
	}
	return merged
}

// safeDiagnosticValue 判定并规范化诊断值：字符串（含命名字符串类型）必须整体匹配
// 安全 token 形态（丢弃而非截断原文），布尔与各宽度数值原样保留；
// nil、容器、结构体等一律丢弃（负向测试固定）。
func safeDiagnosticValue(value any) (any, bool) {
	switch typed := value.(type) {
	case nil:
		return nil, false
	case bool:
		return typed, true
	}
	switch rv := reflect.ValueOf(value); rv.Kind() {
	case reflect.String:
		token := rv.String()
		if len([]rune(token)) > maxDiagnosticValueRunes || !diagnosticTokenPattern.MatchString(token) {
			return nil, false
		}
		// 字符集已排除空格/引号/=等凭据载体，Redact 仅防御 sk- 型密钥恰好通过字符集。
		return Redact(token), true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return value, true
	default:
		return nil, false
	}
}
