// src/pkg/logger/config.go
package logger

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap/zapcore"
)

// Config 模块日志配置（内部使用）
type Config struct {
	Level           string
	Development     bool
	Encoding        string // json, console 或 console_pretty
	ConsoleEncoding string

	// 内部字段（由 Manager 自动设置，用户无需关心）
	moduleName string // 业务模块名称（如：order、auth、user）
	logDir     string // 日志根目录（默认 logs/）

	EnableFile    bool
	EnableConsole bool

	// 文件名格式配置
	EnableLevelInFilename    bool   // 是否包含级别（info/error）
	EnableSequenceInFilename bool   // 是否包含序号（01/02）
	SequenceNumber           string // 序号（如："01"）
	EnableDateInFilename     bool   // 是否包含日期
	DateFormat               string // 日期格式（默认 2006-01-02）

	// 文件切割配置
	MaxSize    int  // 单个文件最大大小（MB）
	MaxBackups int  // 保留旧文件数量
	MaxAge     int  // 保留天数
	Compress   bool // 是否压缩

	// 调用栈配置
	EnableCaller     bool
	EnableStacktrace bool
	StacktraceLevel  string // 从哪个级别开始记录栈（默认 error）
	StacktraceDepth  int    // 堆栈深度限制（0=不限制，建议 5-10）
}

// ManagerConfig 全局管理器配置（所有模块共享）
type ManagerConfig struct {
	BaseLogDir               string `mapstructure:"base_log_dir"` // 固定根目录（默认 logs/）
	Level                    string `mapstructure:"level"`
	AppName                  string `mapstructure:"app_name"`      // 应用名称（自动注入所有日志，空值也注入）
	Encoding                 string `mapstructure:"encoding"`
	ConsoleEncoding          string `mapstructure:"console_encoding"`
	EnableConsole            bool   `mapstructure:"enable_console"`
	EnableLevelInFilename    bool   `mapstructure:"enable_level_in_filename"`
	EnableSequenceInFilename bool   `mapstructure:"enable_sequence_in_filename"`
	EnableDateInFilename     bool   `mapstructure:"enable_date_in_filename"`
	DateFormat               string `mapstructure:"date_format"`
	MaxSize                  int    `mapstructure:"max_size"`
	MaxBackups               int    `mapstructure:"max_backups"`
	MaxAge                   int    `mapstructure:"max_age"`
	Compress                 bool   `mapstructure:"compress"`
	EnableCaller             bool   `mapstructure:"enable_caller"`
	EnableStacktrace         bool   `mapstructure:"enable_stacktrace"`
	StacktraceLevel          string `mapstructure:"stacktrace_level"`
	StacktraceDepth          int    `mapstructure:"stacktrace_depth"` // 堆栈深度（0=不限制）
	LoggerName               string `mapstructure:"logger_name"`
	ModuleNumber             int    `mapstructure:"module_number"`

	// 渲染样式配置（仅对 console_pretty 编码器有效）
	// 可选值：single_line（默认）、key_value
	RenderStyle string `mapstructure:"render_style"`

	// TraceID 配置
	EnableTraceID    bool   `mapstructure:"enable_trace_id"`     // 是否启用 traceID 自动提取
	TraceIDKey       string `mapstructure:"trace_id_key"`        // context 中的 key（默认 "trace_id"）
	TraceIDFieldName string `mapstructure:"trace_id_field_name"` // 日志字段名（默认 "trace_id"）
}

// DefaultManagerConfig 返回默认管理器配置
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		BaseLogDir:               "logs",
		LoggerName:               "logger",
		Level:                    "info",
		Encoding:                 "json",
		EnableConsole:            true,
		EnableLevelInFilename:    true,
		EnableSequenceInFilename: false,
		EnableDateInFilename:     true,
		DateFormat:               "2006-01-02",
		MaxSize:                  100,
		MaxBackups:               3,
		MaxAge:                   28,
		Compress:                 true,
		EnableCaller:             true,
		EnableStacktrace:         true,
		StacktraceLevel:          "error", // 从 error 级别开始记录堆栈
		StacktraceDepth:          5,       // 默认只记录 5 层堆栈，避免日志过长
		EnableTraceID:            true,
		TraceIDKey:               "trace_id",
		TraceIDFieldName:         "trace_id",
	}
}

// ApplyDefaults 将零值字段填充为默认值（原地修改）
// 用于处理配置文件中缺失或为零值的字段
func (c *ManagerConfig) ApplyDefaults() {
	defaults := DefaultManagerConfig()

	// 字符串类型：空字符串视为未配置
	if c.BaseLogDir == "" {
		c.BaseLogDir = defaults.BaseLogDir
	}
	if c.ModuleNumber == 0 {
		c.ModuleNumber = 50
	}
	if c.LoggerName == "" {
		c.LoggerName = defaults.LoggerName
	}
	if c.Level == "" {
		c.Level = defaults.Level
	}
	if c.Encoding == "" {
		c.Encoding = defaults.Encoding
	}
	if c.ConsoleEncoding == "" {
		c.ConsoleEncoding = defaults.ConsoleEncoding
	}
	if c.DateFormat == "" {
		c.DateFormat = defaults.DateFormat
	}
	if c.StacktraceLevel == "" {
		c.StacktraceLevel = defaults.StacktraceLevel
	}
	if c.TraceIDKey == "" {
		c.TraceIDKey = defaults.TraceIDKey
	}
	if c.TraceIDFieldName == "" {
		c.TraceIDFieldName = defaults.TraceIDFieldName
	}

	// 数值类型：0 视为未配置（注意：MaxBackups=0 是合法值，但很少用）
	if c.MaxSize == 0 {
		c.MaxSize = defaults.MaxSize
	}
	if c.MaxBackups == 0 {
		c.MaxBackups = defaults.MaxBackups
	}
	if c.MaxAge == 0 {
		c.MaxAge = defaults.MaxAge
	}

	// 布尔类型：无法判断是否配置，保持原值
	// 如果需要默认值，应在配置文件中显式设置
}

// Validate 验证配置（实现 config.Validator 接口）
func (c *Config) Validate() error {
	// 1. 基础验证
	if c.logDir == "" {
		return fmt.Errorf("[Logger] 日志目录不能为空")
	}

	// 2. 枚举验证
	validLevels := []string{"debug", "info", "warn", "error", "fatal"}
	if !contains(validLevels, c.Level) {
		return fmt.Errorf("[Logger] 日志级别必须是: %v，当前: %s", validLevels, c.Level)
	}

	validEncodings := []string{"json", "console", "console_pretty"}
	if !contains(validEncodings, c.Encoding) {
		return fmt.Errorf("[Logger] 编码格式必须是: %v，当前: %s", validEncodings, c.Encoding)
	}

	// 3. 范围验证
	if c.MaxSize < 1 || c.MaxSize > 10000 {
		return fmt.Errorf("[Logger] 文件大小必须在1-10000MB之间，当前: %d", c.MaxSize)
	}

	if c.MaxBackups < 0 || c.MaxBackups > 100 {
		return fmt.Errorf("[Logger] 备份数量必须在0-100之间，当前: %d", c.MaxBackups)
	}

	if c.MaxAge < 1 || c.MaxAge > 365 {
		return fmt.Errorf("[Logger] 保留天数必须在1-365之间，当前: %d", c.MaxAge)
	}

	// 4. 业务逻辑验证
	if c.EnableDateInFilename && c.DateFormat == "" {
		return fmt.Errorf("[Logger] 启用日期文件名时必须指定日期格式")
	}

	return nil
}

// ParseLevel 解析日志级别字符串
func ParseLevel(level string) zapcore.Level {
	switch level {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	case "fatal":
		return zapcore.FatalLevel
	default:
		return zapcore.InfoLevel
	}
}

// Validate 验证 ManagerConfig 配置
func (c ManagerConfig) Validate() error {
	// 验证日志级别
	validLevels := []string{"debug", "info", "warn", "error", "fatal"}
	if !contains(validLevels, c.Level) {
		return fmt.Errorf("无效的日志级别: %s (有效值: %v)", c.Level, validLevels)
	}

	// 验证编码格式
	validEncodings := []string{"json", "console", "console_pretty"}
	if !contains(validEncodings, c.Encoding) {
		return fmt.Errorf("无效的日志编码: %s (有效值: %v)", c.Encoding, validEncodings)
	}

	// 验证文件大小
	if c.MaxSize < 1 || c.MaxSize > 10000 {
		return fmt.Errorf("MaxSize 必须在 1-10000 MB 之间，当前: %d", c.MaxSize)
	}

	// 验证备份数量
	if c.MaxBackups < 0 || c.MaxBackups > 1000 {
		return fmt.Errorf("MaxBackups 必须在 0-1000 之间，当前: %d", c.MaxBackups)
	}

	// 验证保留天数
	if c.MaxAge < 0 || c.MaxAge > 3650 {
		return fmt.Errorf("MaxAge 必须在 0-3650 天之间，当前: %d", c.MaxAge)
	}

	// 验证栈追踪级别
	if !contains(validLevels, c.StacktraceLevel) {
		return fmt.Errorf("无效的栈追踪级别: %s (有效值: %v)", c.StacktraceLevel, validLevels)
	}

	return nil
}

// contains 检查字符串切片是否包含指定字符串
func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

// getModuleLogDir 获取模块日志目录（内部方法）
// 返回：logs/order/ 或 logs/auth/
func (c Config) getModuleLogDir() string {
	if c.moduleName == "" {
		return c.logDir
	}
	return filepath.Join(c.logDir, c.moduleName)
}

// getInfoFilePath 获取 Info 日志完整路径（内部方法）
func (c Config) getInfoFilePath() string {
	return c.buildFilePath("info")
}

// getErrorFilePath 获取 Error 日志完整路径（内部方法）
func (c Config) getErrorFilePath() string {
	return c.buildFilePath("error")
}

// buildFilePath 构建日志文件路径（内部方法）
// 支持格式：
//   - logs/order/order.log（仅模块名）
//   - logs/order/order-info.log（模块名 + 级别）
//   - logs/order/order-info-01.log（模块名 + 级别 + 序号）
//   - logs/order/order-info-2024-12-19.log（模块名 + 级别 + 日期）
//   - logs/order/order-info-01-2024-12-19.log（完整格式）
func (c Config) buildFilePath(level string) string {
	parts := []string{c.moduleName}

	// 添加级别
	if c.EnableLevelInFilename {
		parts = append(parts, level)
	}

	// 添加序号
	if c.EnableSequenceInFilename && c.SequenceNumber != "" {
		parts = append(parts, c.SequenceNumber)
	}

	// 添加日期
	if c.EnableDateInFilename {
		date := time.Now().Format(c.DateFormat)
		parts = append(parts, date)
	}

	// 组合文件名
	filename := strings.Join(parts, "-")

	// 返回完整路径
	return filepath.Join(c.getModuleLogDir(), filename+".log")
}
