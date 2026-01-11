package logger

import (
	"encoding/base64"
	"math"
	"sync"
	"time"
	"unicode/utf8"

	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

// RenderStyle 日志渲染样式
type RenderStyle string

const (
	// RenderStyleSingleLine 单行渲染（默认）
	// 格式: [🔵INFO]  |  2025-12-23T01:10:01.165+0800  |  message  |  [module]  |  file:line  |  trace-id  |  {"key":"value"}
	RenderStyleSingleLine RenderStyle = "single_line"

	// RenderStyleKeyValue 键值对渲染（多行，适合小屏幕）
	// 格式:
	//   🟢 DEBU | 2025-12-23 01:10:01.165
	//     trace: -
	//     module: gin-route
	//     caller: logger/manager.go:316
	//     message: [GIN-debug] GET / --> ...
	RenderStyleKeyValue RenderStyle = "key_value"
)

// ParseRenderStyle 解析渲染样式字符串
func ParseRenderStyle(s string) RenderStyle {
	switch s {
	case "key_value":
		return RenderStyleKeyValue
	case "single_line", "":
		return RenderStyleSingleLine
	default:
		return RenderStyleSingleLine // 默认单行
	}
}

var _prettyEncoderPool = sync.Pool{
	New: func() interface{} {
		return &PrettyConsoleEncoder{}
	},
}

func getPrettyEncoder() *PrettyConsoleEncoder {
	return _prettyEncoderPool.Get().(*PrettyConsoleEncoder)
}

func putPrettyEncoder(enc *PrettyConsoleEncoder) {
	enc.EncoderConfig = nil
	enc.buf = nil
	_prettyEncoderPool.Put(enc)
}

// PrettyConsoleEncoder 美化的控制台编码器
// 支持多种渲染样式：单行、键值对等
type PrettyConsoleEncoder struct {
	*zapcore.EncoderConfig
	buf         *buffer.Buffer
	moduleName  string      // 捕获的模块名
	traceID     string      // 捕获的 traceID
	renderStyle RenderStyle // 渲染样式
}

// NewPrettyConsoleEncoder 创建美化控制台编码器（默认单行样式）
func NewPrettyConsoleEncoder(cfg zapcore.EncoderConfig) zapcore.Encoder {
	return &PrettyConsoleEncoder{
		EncoderConfig: &cfg,
		renderStyle:   RenderStyleSingleLine, // 默认单行
	}
}

// NewPrettyConsoleEncoderWithStyle 创建指定样式的美化控制台编码器
func NewPrettyConsoleEncoderWithStyle(cfg zapcore.EncoderConfig, style RenderStyle) zapcore.Encoder {
	return &PrettyConsoleEncoder{
		EncoderConfig: &cfg,
		renderStyle:   style,
	}
}

// Clone 克隆编码器
func (enc *PrettyConsoleEncoder) Clone() zapcore.Encoder {
	clone := getPrettyEncoder()
	clone.EncoderConfig = enc.EncoderConfig
	clone.buf = buffer.NewPool().Get()
	clone.moduleName = enc.moduleName   // 继承 module
	clone.traceID = enc.traceID         // 继承 traceID
	clone.renderStyle = enc.renderStyle // 继承渲染样式
	return clone
}

// EncodeEntry 编码日志条目（根据渲染样式分发）
func (enc *PrettyConsoleEncoder) EncodeEntry(entry zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	switch enc.renderStyle {
	case RenderStyleKeyValue:
		return enc.encodeKeyValue(entry, fields)
	case RenderStyleSingleLine:
		fallthrough
	default:
		return enc.encodeSingleLine(entry, fields)
	}
}

// encodeSingleLine 单行渲染
// 格式: [🔵INFO]  |  2025-12-20T09:14:58.575+0800  |  message  |  [module]  |  file:line  |  trace-id  |  {"key":"value"}
func (enc *PrettyConsoleEncoder) encodeSingleLine(entry zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	final := buffer.NewPool().Get()

	// 1. 级别 [🔵INFO] (固定10字符，包含emoji)
	final.AppendString("[")
	final.AppendString(enc.levelWithEmoji(entry.Level))
	final.AppendString("]")

	// 2. 分隔符
	final.AppendString("  |  ")

	// 3. 完整时间戳 2025-12-20T09:14:58.575+0800 (固定29字符)
	final.AppendString(entry.Time.Format("2006-01-02T15:04:05.000-0700"))

	// 4. 分隔符
	final.AppendString("  |  ")

	// 5. 消息 (不限制长度)
	final.AppendString(entry.Message)

	// 6. 分隔符
	final.AppendString("  |  ")

	// 7. 模块名 [order] (固定25字符，包含方括号)
	moduleName := enc.extractModule(fields)
	if moduleName == "unknown" {
		moduleName = enc.moduleName
		if moduleName == "" {
			moduleName = "unknown"
		}
	}
	enc.appendPaddedModule(final, moduleName, 25)

	// 8. 分隔符
	final.AppendString("  |  ")

	// 9. 文件位置 order/manager.go:123 (固定50字符)
	if entry.Caller.Defined {
		enc.appendPadded(final, entry.Caller.TrimmedPath(), 50, false)
	} else {
		enc.appendPadded(final, "", 50, false)
	}

	// 10. 分隔符
	final.AppendString("  |  ")

	// 11. TraceID (固定16字符，右对齐或"-")
	traceID := enc.extractTraceID(fields)
	if traceID == "" {
		traceID = enc.traceID
	}
	if traceID != "" {
		enc.appendPadded(final, traceID, 16, false) // 左对齐
	} else {
		enc.appendPadded(final, "-", 16, true) // 居中
	}

	// 12. 分隔符 + 字段（JSON格式）
	if len(fields) > 0 {
		final.AppendString("  |  ")
		enc.appendFieldsAsJSON(final, fields)
	}

	final.AppendString("\n")

	// 堆栈信息（从 entry.Stack 或 fields 中提取）
	stackTrace := entry.Stack
	if stackTrace == "" {
		stackTrace = enc.extractStack(fields)
	}
	if stackTrace != "" {
		final.AppendString(stackTrace)
		final.AppendString("\n")
	}

	return final, nil
}

// encodeKeyValue 键值对渲染（多行）
// 格式:
//
//	🟢 DEBU | 2025-12-23 01:10:01.165
//	  trace: -
//	  module: gin-route
//	  caller: logger/manager.go:316
//	  message: [GIN-debug] GET / --> ...
//	  fields: {"key":"value"}
func (enc *PrettyConsoleEncoder) encodeKeyValue(entry zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	final := buffer.NewPool().Get()

	// 第1行：级别 + 简化时间
	final.AppendString(enc.levelWithEmojiShort(entry.Level))
	final.AppendString(" ")
	final.AppendString(enc.levelNameShort(entry.Level))
	final.AppendString(" | ")
	final.AppendString(entry.Time.Format("2006-01-02 15:04:05.000"))
	final.AppendString("\n")

	// 第2行：trace
	traceID := enc.extractTraceID(fields)
	if traceID == "" {
		traceID = enc.traceID
	}
	if traceID == "" {
		traceID = "-"
	}
	final.AppendString("  trace: ")
	final.AppendString(traceID)
	final.AppendString("\n")

	// 第3行：module
	moduleName := enc.extractModule(fields)
	if moduleName == "unknown" {
		moduleName = enc.moduleName
		if moduleName == "" {
			moduleName = "unknown"
		}
	}
	final.AppendString("  module: ")
	final.AppendString(moduleName)
	final.AppendString("\n")

	// 第4行：caller
	final.AppendString("  caller: ")
	if entry.Caller.Defined {
		final.AppendString(entry.Caller.TrimmedPath())
	} else {
		final.AppendString("-")
	}
	final.AppendString("\n")

	// 第5行：message
	final.AppendString("  message: ")
	final.AppendString(entry.Message)
	final.AppendString("\n")

	// 第6行（可选）：fields
	if len(fields) > 0 && enc.hasNonMetaFields(fields) {
		final.AppendString("  fields: ")
		enc.appendFieldsAsJSON(final, fields)
		final.AppendString("\n")
	}

	// 栈追踪（从 entry.Stack 或 fields 中提取）
	stackTrace := entry.Stack
	if stackTrace == "" {
		stackTrace = enc.extractStack(fields)
	}
	if stackTrace != "" {
		final.AppendString("  stack:\n")
		// 给每行添加缩进
		enc.appendIndentedStack(final, stackTrace, "    ")
	}

	return final, nil
}

// appendPadded 追加固定宽度的字符串（左对齐或居中）
func (enc *PrettyConsoleEncoder) appendPadded(buf *buffer.Buffer, s string, width int, center bool) {
	sLen := len(s)
	if sLen >= width {
		buf.AppendString(s[:width])
		return
	}

	padding := width - sLen
	if center {
		leftPad := padding / 2
		rightPad := padding - leftPad
		for i := 0; i < leftPad; i++ {
			buf.AppendByte(' ')
		}
		buf.AppendString(s)
		for i := 0; i < rightPad; i++ {
			buf.AppendByte(' ')
		}
	} else {
		// 左对齐
		buf.AppendString(s)
		for i := 0; i < padding; i++ {
			buf.AppendByte(' ')
		}
	}
}

// appendPaddedModule 追加固定宽度的模块名（包含方括号）
func (enc *PrettyConsoleEncoder) appendPaddedModule(buf *buffer.Buffer, moduleName string, totalWidth int) {
	// [moduleName] 总长度 = len(moduleName) + 2
	moduleStr := "[" + moduleName + "]"
	enc.appendPadded(buf, moduleStr, totalWidth, false)
}

// levelWithEmoji 带 Emoji 的级别（完整版，用于单行）
func (enc *PrettyConsoleEncoder) levelWithEmoji(level zapcore.Level) string {
	switch level {
	case zapcore.DebugLevel:
		return "🟢DEBU"
	case zapcore.InfoLevel:
		return "🔵INFO"
	case zapcore.WarnLevel:
		return "🟡WARN"
	case zapcore.ErrorLevel:
		return "🔴ERRO"
	case zapcore.DPanicLevel:
		return "🟠DPAN"
	case zapcore.PanicLevel:
		return "🟣PANI"
	case zapcore.FatalLevel:
		return "💀FATA"
	default:
		return level.CapitalString()
	}
}

// levelWithEmojiShort 只返回 Emoji（用于键值对渲染）
func (enc *PrettyConsoleEncoder) levelWithEmojiShort(level zapcore.Level) string {
	switch level {
	case zapcore.DebugLevel:
		return "🟢"
	case zapcore.InfoLevel:
		return "🔵"
	case zapcore.WarnLevel:
		return "🟡"
	case zapcore.ErrorLevel:
		return "🔴"
	case zapcore.DPanicLevel:
		return "🟠"
	case zapcore.PanicLevel:
		return "🟣"
	case zapcore.FatalLevel:
		return "💀"
	default:
		return "⚪"
	}
}

// levelNameShort 级别名称（4字符）
func (enc *PrettyConsoleEncoder) levelNameShort(level zapcore.Level) string {
	switch level {
	case zapcore.DebugLevel:
		return "DEBU"
	case zapcore.InfoLevel:
		return "INFO"
	case zapcore.WarnLevel:
		return "WARN"
	case zapcore.ErrorLevel:
		return "ERRO"
	case zapcore.DPanicLevel:
		return "DPAN"
	case zapcore.PanicLevel:
		return "PANI"
	case zapcore.FatalLevel:
		return "FATA"
	default:
		return level.CapitalString()
	}
}

// hasNonMetaFields 检查是否有非元数据字段
func (enc *PrettyConsoleEncoder) hasNonMetaFields(fields []zapcore.Field) bool {
	for _, field := range fields {
		if field.Key != "trace_id" && field.Key != "module" && field.Key != "stack" {
			return true
		}
	}
	return false
}

// extractTraceID 从字段中提取 trace_id
func (enc *PrettyConsoleEncoder) extractTraceID(fields []zapcore.Field) string {
	for _, field := range fields {
		if field.Key == "trace_id" {
			if field.Type == zapcore.StringType {
				return field.String
			}
		}
	}
	return ""
}

// extractModule 从字段中提取 module
func (enc *PrettyConsoleEncoder) extractModule(fields []zapcore.Field) string {
	for _, field := range fields {
		if field.Key == "module" {
			if field.Type == zapcore.StringType {
				return field.String
			}
		}
	}
	return "unknown"
}

// extractStack 从字段中提取 stack
func (enc *PrettyConsoleEncoder) extractStack(fields []zapcore.Field) string {
	for _, field := range fields {
		if field.Key == "stack" {
			if field.Type == zapcore.StringType {
				return field.String
			}
		}
	}
	return ""
}

// appendIndentedStack 追加带缩进的堆栈信息
func (enc *PrettyConsoleEncoder) appendIndentedStack(buf *buffer.Buffer, stack string, indent string) {
	lines := 0
	for i := 0; i < len(stack); i++ {
		if i == 0 || stack[i-1] == '\n' {
			buf.AppendString(indent)
		}
		buf.AppendByte(stack[i])
		if stack[i] == '\n' {
			lines++
		}
	}
	// 确保以换行结尾
	if len(stack) > 0 && stack[len(stack)-1] != '\n' {
		buf.AppendString("\n")
	}
}

// appendFieldsAsJSON 将字段编码为 JSON
func (enc *PrettyConsoleEncoder) appendFieldsAsJSON(buf *buffer.Buffer, fields []zapcore.Field) {
	buf.AppendByte('{')
	first := true
	for _, field := range fields {
		// 跳过内部字段（trace_id, module, stack）
		if field.Key == "trace_id" || field.Key == "module" || field.Key == "stack" {
			continue
		}

		if !first {
			buf.AppendByte(',')
		}
		first = false

		// 字段名
		buf.AppendByte('"')
		buf.AppendString(field.Key)
		buf.AppendString(`":`)

		// 字段值
		enc.appendFieldValue(buf, field)
	}
	buf.AppendByte('}')
}

// appendFieldValue 追加字段值
func (enc *PrettyConsoleEncoder) appendFieldValue(buf *buffer.Buffer, field zapcore.Field) {
	switch field.Type {
	case zapcore.StringType:
		buf.AppendByte('"')
		enc.safeAddString(buf, field.String)
		buf.AppendByte('"')

	case zapcore.Int64Type, zapcore.Int32Type, zapcore.Int16Type, zapcore.Int8Type:
		buf.AppendInt(field.Integer)

	case zapcore.Uint64Type, zapcore.Uint32Type, zapcore.Uint16Type, zapcore.Uint8Type:
		buf.AppendUint(uint64(field.Integer))

	case zapcore.Float64Type:
		buf.AppendFloat(math.Float64frombits(uint64(field.Integer)), 64)

	case zapcore.Float32Type:
		buf.AppendFloat(float64(math.Float32frombits(uint32(field.Integer))), 32)

	case zapcore.BoolType:
		buf.AppendBool(field.Integer == 1)

	case zapcore.DurationType:
		buf.AppendInt(field.Integer)

	case zapcore.TimeType:
		if field.Interface != nil {
			buf.AppendByte('"')
			buf.AppendTime(time.Unix(0, field.Integer), time.RFC3339)
			buf.AppendByte('"')
		} else {
			buf.AppendInt(field.Integer)
		}

	case zapcore.BinaryType:
		buf.AppendByte('"')
		buf.AppendString(base64.StdEncoding.EncodeToString(field.Interface.([]byte)))
		buf.AppendByte('"')

	case zapcore.ErrorType:
		// 处理 error 类型
		if field.Interface != nil {
			buf.AppendByte('"')
			if err, ok := field.Interface.(error); ok {
				enc.safeAddString(buf, err.Error())
			} else {
				enc.safeAddString(buf, "unknown error")
			}
			buf.AppendByte('"')
		} else {
			buf.AppendString(`null`)
		}

	case zapcore.ReflectType:
		buf.AppendString(`"<reflect>"`)

	default:
		buf.AppendString(`null`)
	}
}

// safeAddString 安全地添加字符串（转义特殊字符）
func (enc *PrettyConsoleEncoder) safeAddString(buf *buffer.Buffer, s string) {
	for i := 0; i < len(s); {
		if b := s[i]; b < utf8.RuneSelf {
			i++
			switch b {
			case '\\', '"':
				buf.AppendByte('\\')
				buf.AppendByte(b)
			case '\n':
				buf.AppendByte('\\')
				buf.AppendByte('n')
			case '\r':
				buf.AppendByte('\\')
				buf.AppendByte('r')
			case '\t':
				buf.AppendByte('\\')
				buf.AppendByte('t')
			default:
				buf.AppendByte(b)
			}
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			buf.AppendString(`\ufffd`)
			i++
			continue
		}
		buf.AppendString(s[i : i+size])
		i += size
	}
}

// 以下方法实现 zapcore.ObjectEncoder 接口（AddString、AddInt 等）
// 这些方法用于字段编码时被调用

func (enc *PrettyConsoleEncoder) AddArray(key string, arr zapcore.ArrayMarshaler) error {
	return nil
}

func (enc *PrettyConsoleEncoder) AddObject(key string, obj zapcore.ObjectMarshaler) error {
	return nil
}

func (enc *PrettyConsoleEncoder) AddBinary(key string, value []byte) {
}

func (enc *PrettyConsoleEncoder) AddByteString(key string, value []byte) {
}

func (enc *PrettyConsoleEncoder) AddBool(key string, value bool) {
}

func (enc *PrettyConsoleEncoder) AddComplex128(key string, value complex128) {
}

func (enc *PrettyConsoleEncoder) AddComplex64(key string, value complex64) {
}

func (enc *PrettyConsoleEncoder) AddDuration(key string, value time.Duration) {
}

func (enc *PrettyConsoleEncoder) AddFloat64(key string, value float64) {
}

func (enc *PrettyConsoleEncoder) AddFloat32(key string, value float32) {
}

func (enc *PrettyConsoleEncoder) AddInt(key string, value int) {
}

func (enc *PrettyConsoleEncoder) AddInt64(key string, value int64) {
}

func (enc *PrettyConsoleEncoder) AddInt32(key string, value int32) {
}

func (enc *PrettyConsoleEncoder) AddInt16(key string, value int16) {
}

func (enc *PrettyConsoleEncoder) AddInt8(key string, value int8) {
}

func (enc *PrettyConsoleEncoder) AddString(key, value string) {
	// 捕获 module 和 trace_id 字段
	if key == "module" {
		enc.moduleName = value
	} else if key == "trace_id" {
		enc.traceID = value
	}
}

func (enc *PrettyConsoleEncoder) AddTime(key string, value time.Time) {
}

func (enc *PrettyConsoleEncoder) AddUint(key string, value uint) {
}

func (enc *PrettyConsoleEncoder) AddUint64(key string, value uint64) {
}

func (enc *PrettyConsoleEncoder) AddUint32(key string, value uint32) {
}

func (enc *PrettyConsoleEncoder) AddUint16(key string, value uint16) {
}

func (enc *PrettyConsoleEncoder) AddUint8(key string, value uint8) {
}

func (enc *PrettyConsoleEncoder) AddUintptr(key string, value uintptr) {
}

func (enc *PrettyConsoleEncoder) AddReflected(key string, value interface{}) error {
	return nil
}

func (enc *PrettyConsoleEncoder) OpenNamespace(key string) {
}
