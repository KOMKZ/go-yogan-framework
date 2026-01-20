package logger

import (
	"encoding/base64"
	"math"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

// RenderStyle logging rendering style
type RenderStyle string

const (
	// Render single line (default)
	// Format: [🔵INFO] | 2025-12-23T01:10:01.165+0800 | message | [module] | file:line | trace-id | {"key":"value"}
	RenderStyleSingleLine RenderStyle = "single_line"

	// RenderStyleKeyValue Key-value pair rendering (multi-line, suitable for small screens)
	// Format:
	//   🟢 DEBU | 2025-12-23 01:10:01.165
	//     trace: -
	//     module: gin-route
	//     caller: logger/manager.go:316
	//     message: [GIN-debug] GET / --> ...
	RenderStyleKeyValue RenderStyle = "key_value"

	// RenderStyleModernCompact Modern compact style
	// Format: 14:30:45 │ INFO  │ HTTP server started                          │ yogan      {"key":"value"}
	// Features: time-saving, Box Drawing separators, fixed column width, clear hierarchy
	RenderStyleModernCompact RenderStyle = "modern_compact"
)

// ParseRenderStyle parse rendering style string
func ParseRenderStyle(s string) RenderStyle {
	switch s {
	case "key_value":
		return RenderStyleKeyValue
	case "modern_compact":
		return RenderStyleModernCompact
	case "single_line", "":
		return RenderStyleSingleLine
	default:
		return RenderStyleSingleLine // Default single line
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

// PrettyConsoleEncoder美化后的控制台编码器
// Supports multiple rendering styles: single line, key-value pairs, etc.
type PrettyConsoleEncoder struct {
	*zapcore.EncoderConfig
	buf         *buffer.Buffer
	moduleName  string      // captured module name
	traceID     string      // captured traceID
	renderStyle RenderStyle // Render style
}

// Create a pretty console encoder (default single-line style)
func NewPrettyConsoleEncoder(cfg zapcore.EncoderConfig) zapcore.Encoder {
	return &PrettyConsoleEncoder{
		EncoderConfig: &cfg,
		renderStyle:   RenderStyleSingleLine, // Default single line
	}
}

// Create a pretty console encoder with specified style
func NewPrettyConsoleEncoderWithStyle(cfg zapcore.EncoderConfig, style RenderStyle) zapcore.Encoder {
	return &PrettyConsoleEncoder{
		EncoderConfig: &cfg,
		renderStyle:   style,
	}
}

// Clone the encoder
func (enc *PrettyConsoleEncoder) Clone() zapcore.Encoder {
	clone := getPrettyEncoder()
	clone.EncoderConfig = enc.EncoderConfig
	clone.buf = buffer.NewPool().Get()
	clone.moduleName = enc.moduleName   // inherit module
	clone.traceID = enc.traceID         // Inherit traceID
	clone.renderStyle = enc.renderStyle // Inherit rendering style
	return clone
}

// EncodeEntry encode log entry (distribute based on rendering style)
func (enc *PrettyConsoleEncoder) EncodeEntry(entry zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	switch enc.renderStyle {
	case RenderStyleKeyValue:
		return enc.encodeKeyValue(entry, fields)
	case RenderStyleModernCompact:
		return enc.encodeModernCompact(entry, fields)
	case RenderStyleSingleLine:
		fallthrough
	default:
		return enc.encodeSingleLine(entry, fields)
	}
}

// encodeSingleLine Single-line rendering
// Format: [🔵INFO] | 2025-12-20T09:14:58.575+0800 | message | [module] | file:line | trace-id | {"key":"value"}
func (enc *PrettyConsoleEncoder) encodeSingleLine(entry zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	final := buffer.NewPool().Get()

	// Level [🔵INFO] (fixed 10 characters, including emoji)
	final.AppendString("[")
	final.AppendString(enc.levelWithEmoji(entry.Level))
	final.AppendString("]")

	// 2. delimiter
	final.AppendString("  |  ")

	// 3. Full timestamp 2025-12-20T09:14:58.575+0800 (fixed 29 characters)
	final.AppendString(entry.Time.Format("2006-01-02T15:04:05.000-0700"))

	// 4. delimiter
	final.AppendString("  |  ")

	// 5. Message (unlimited length)
	final.AppendString(entry.Message)

	// English: 6. Delimiter
	final.AppendString("  |  ")

	// Module name [order] (fixed 25 characters, including brackets)
	moduleName := enc.extractModule(fields)
	if moduleName == "unknown" {
		moduleName = enc.moduleName
		if moduleName == "" {
			moduleName = "unknown"
		}
	}
	enc.appendPaddedModule(final, moduleName, 25)

	// 8. delimiter
	final.AppendString("  |  ")

	// File position order/manager.go:123 (fixed at 50 characters)
	if entry.Caller.Defined {
		enc.appendPadded(final, entry.Caller.TrimmedPath(), 50, false)
	} else {
		enc.appendPadded(final, "", 50, false)
	}

	// 10. Delimiter
	final.AppendString("  |  ")

	// 11. TraceID (fixed 16 characters, right-aligned or "-")
	traceID := enc.extractTraceID(fields)
	if traceID == "" {
		traceID = enc.traceID
	}
	if traceID != "" {
		enc.appendPadded(final, traceID, 16, false) // left align
	} else {
		enc.appendPadded(final, "-", 16, true) // centered
	}

	// delimiter + field (JSON format)
	if len(fields) > 0 {
		final.AppendString("  |  ")
		enc.appendFieldsAsJSON(final, fields)
	}

	final.AppendString("\n")

	// Stack information (extracted from entry.Stack or fields)
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

// encodeKeyValue Render key-value pairs (multi-line)
// Format:
//
//	🟢 DEBU | 2025-12-23 01:10:01.165
//	  trace: -
//	  module: gin-route
//	  caller: logger/manager.go:316
//	  message: [GIN-debug] GET / --> ...
//	  fields: {"key":"value"}
func (enc *PrettyConsoleEncoder) encodeKeyValue(entry zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	final := buffer.NewPool().Get()

	// English: Line 1: Level + Simplified Time
	final.AppendString(enc.levelWithEmojiShort(entry.Level))
	final.AppendString(" ")
	final.AppendString(enc.levelNameShort(entry.Level))
	final.AppendString(" | ")
	final.AppendString(entry.Time.Format("2006-01-02 15:04:05.000"))
	final.AppendString("\n")

	// Line 2: trace
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

	// English: Line 3: module
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

	// English: Line 4: caller
	final.AppendString("  caller: ")
	if entry.Caller.Defined {
		final.AppendString(entry.Caller.TrimmedPath())
	} else {
		final.AppendString("-")
	}
	final.AppendString("\n")

	// Line 5: message
	final.AppendString("  message: ")
	final.AppendString(entry.Message)
	final.AppendString("\n")

	// English: (Optional) line 6: fields
	if len(fields) > 0 && enc.hasNonMetaFields(fields) {
		final.AppendString("  fields: ")
		enc.appendFieldsAsJSON(final, fields)
		final.AppendString("\n")
	}

	// stack trace (extracted from entry.Stack or fields)
	stackTrace := entry.Stack
	if stackTrace == "" {
		stackTrace = enc.extractStack(fields)
	}
	if stackTrace != "" {
		final.AppendString("  stack:\n")
		// Add indentation to each line
		enc.appendIndentedStack(final, stackTrace, "    ")
	}

	return final, nil
}

// encodeModernCompact Modern compact style rendering
// Format: 14:30:45 │ INFO  │ HTTP server started                          │ yogan       {"key":"value"}
// Features: time-saving, Box Drawing separators, fixed column width, clear hierarchy
func (enc *PrettyConsoleEncoder) encodeModernCompact(entry zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	final := buffer.NewPool().Get()

	// Timestamp HH:MM:SS (8 characters)
	final.AppendString(entry.Time.Format("15:04:05"))

	// 2. Separator (using Box Drawing characters)
	final.AppendString(" │ ")

	// Level (5 character fixed width, left aligned)
	enc.appendPadded(final, enc.levelNameCompact(entry.Level), 5, false)

	// 4. delimiter
	final.AppendString(" │ ")

	// 5. Message (fixed 45 character width)
	enc.appendPaddedMessage(final, entry.Message, 45)

	// 6. delimiter
	final.AppendString(" │ ")

	// Module name (fixed width of 12 characters)
	moduleName := enc.extractModule(fields)
	if moduleName == "unknown" {
		moduleName = enc.moduleName
		if moduleName == "" {
			moduleName = "-"
		}
	}
	enc.appendPadded(final, moduleName, 12, false)

	// 8. Fields (JSON format, optional)
	if len(fields) > 0 && enc.hasNonMetaFields(fields) {
		final.AppendString(" ")
		enc.appendFieldsAsJSON(final, fields)
	}

	final.AppendString("\n")

	// stack information (extracted from entry.Stack or fields)
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

// levelNameCompact Level name (5 characters, for Modern Compact)
func (enc *PrettyConsoleEncoder) levelNameCompact(level zapcore.Level) string {
	switch level {
	case zapcore.DebugLevel:
		return "DEBUG"
	case zapcore.InfoLevel:
		return "INFO"
	case zapcore.WarnLevel:
		return "WARN"
	case zapcore.ErrorLevel:
		return "ERROR"
	case zapcore.DPanicLevel:
		return "DPANI"
	case zapcore.PanicLevel:
		return "PANIC"
	case zapcore.FatalLevel:
		return "FATAL"
	default:
		return level.CapitalString()
	}
}

// calculate the terminal display width of a string
// Chinese, Japanese, Korean characters and full-width characters take up 2 character widths
func stringDisplayWidth(s string) int {
	width := 0
	for _, r := range s {
		if isWideChar(r) {
			width += 2
		} else {
			width += 1
		}
	}
	return width
}

// determines whether it is a wide character (full-width character)
// Includes: CJK characters, full-width punctuation, emojis etc.
func isWideChar(r rune) bool {
	// Unified CJK Han Characters
	if r >= 0x4E00 && r <= 0x9FFF {
		return true
	}
	// CJK Extended A
	if r >= 0x3400 && r <= 0x4DBF {
		return true
	}
	// CJK Extended-B Range F
	if r >= 0x20000 && r <= 0x2CEAF {
		return true
	}
	// Full-width ASCII and punctuation
	if r >= 0xFF01 && r <= 0xFF60 {
		return true
	}
	// Japanese hiragana and katakana
	if r >= 0x3040 && r <= 0x30FF {
		return true
	}
	// Korean syllable
	if r >= 0xAC00 && r <= 0xD7AF {
		return true
	}
	// Chinese punctuation symbols
	if r >= 0x3000 && r <= 0x303F {
		return true
	}
	// Emoji (part)
	if r >= 0x1F300 && r <= 0x1F9FF {
		return true
	}
	// Use the unicode package to detect East Asian wide characters
	if unicode.Is(unicode.Han, r) {
		return true
	}
	return false
}

// truncate string to specified display width
func truncateToDisplayWidth(s string, maxWidth int) (result string, actualWidth int) {
	width := 0
	for i, r := range s {
		charWidth := 1
		if isWideChar(r) {
			charWidth = 2
		}
		if width+charWidth > maxWidth {
			return s[:i], width
		}
		width += charWidth
	}
	return s, width
}

// Append fixed-width message (truncate or pad, supports Chinese)
func (enc *PrettyConsoleEncoder) appendPaddedMessage(buf *buffer.Buffer, msg string, width int) {
	displayWidth := stringDisplayWidth(msg)

	if displayWidth >= width {
		// truncate and add ellipsis
		if width > 3 {
			truncated, actualWidth := truncateToDisplayWidth(msg, width-3)
			buf.AppendString(truncated)
			buf.AppendString("...")
			// Fill remaining spaces (if any)
			padding := width - actualWidth - 3
			for i := 0; i < padding; i++ {
				buf.AppendByte(' ')
			}
		} else {
			truncated, _ := truncateToDisplayWidth(msg, width)
			buf.AppendString(truncated)
		}
		return
	}

	// left align, pad with spaces
	buf.AppendString(msg)
	for i := 0; i < width-displayWidth; i++ {
		buf.AppendByte(' ')
	}
}

// appendPadded appends fixed-width strings (left-aligned or centered)
func (enc *PrettyConsoleEncoder) appendPadded(buf *buffer.Buffer, s string, width int, center bool) {
	displayWidth := stringDisplayWidth(s)
	if displayWidth >= width {
		// Truncate to specified width
		truncated, _ := truncateToDisplayWidth(s, width)
		buf.AppendString(truncated)
		return
	}

	padding := width - displayWidth
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
		// left align
		buf.AppendString(s)
		for i := 0; i < padding; i++ {
			buf.AppendByte(' ')
		}
	}
}

// Append padded module name (including brackets)
func (enc *PrettyConsoleEncoder) appendPaddedModule(buf *buffer.Buffer, moduleName string, totalWidth int) {
	// [module name] total length = len(module_name) + 2
	moduleStr := "[" + moduleName + "]"
	enc.appendPadded(buf, moduleStr, totalWidth, false)
}

// levelWithEmoji Level with Emoji (full version, for single line)
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

// levelWithEmojiShort only returns Emoji (for key-value pair rendering)
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

// levelNameShort Level name (4 characters)
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

// checks if there are non-metadata fields
func (enc *PrettyConsoleEncoder) hasNonMetaFields(fields []zapcore.Field) bool {
	for _, field := range fields {
		if field.Key != "trace_id" && field.Key != "module" && field.Key != "stack" {
			return true
		}
	}
	return false
}

// extract trace_id from fields
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

// extractModule extracts module from fields
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

// extractStack extract stack from fields
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

// appendIndentedStack Append indented stack information
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
	// Ensure line ending with a newline
	if len(stack) > 0 && stack[len(stack)-1] != '\n' {
		buf.AppendString("\n")
	}
}

// appendFieldsAsJSON encodes fields as JSON
func (enc *PrettyConsoleEncoder) appendFieldsAsJSON(buf *buffer.Buffer, fields []zapcore.Field) {
	buf.AppendByte('{')
	first := true
	for _, field := range fields {
		// Skip internal fields (trace_id, module, stack)
		if field.Key == "trace_id" || field.Key == "module" || field.Key == "stack" {
			continue
		}

		if !first {
			buf.AppendByte(',')
		}
		first = false

		// field name
		buf.AppendByte('"')
		buf.AppendString(field.Key)
		buf.AppendString(`":`)

		// field value
		enc.appendFieldValue(buf, field)
	}
	buf.AppendByte('}')
}

// Append field value
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
		// Handle error types
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

// safelyAddString (escape special characters)
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

// The following method implements the zapcore.ObjectEncoder interface (AddString, AddInt, etc.)
// These methods are called when field encoding is performed

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
	// Capture the module and trace_id fields
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
