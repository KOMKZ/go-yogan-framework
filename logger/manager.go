// src/pkg/logger/manager.go
package logger

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Manager Logger 管理器（管理多个 Logger 实例）
type Manager struct {
	baseConfig ManagerConfig
	loggers    map[string]*CtxZapLogger        // 模块名 -> CtxZapLogger 实例
	zapLoggers map[string]*zap.Logger          // 模块名 -> 底层 zap.Logger 实例
	writers    map[string][]*lumberjack.Logger // 模块名 -> 文件写入器（用于关闭）
	mu         sync.RWMutex                    // 并发安全
}

var (
	globalManager *Manager
	managerOnce   sync.Once
)

// NewManager 创建独立的 Manager 实例（支持多实例场景）
// 用法：
//
//	appManager := logger.NewManager(cfg)
//	appManager.Info("order", "Order creation")
//
// NewManager 创建独立的 Manager 实例
// cfg 中的零值字段会自动填充为默认值
func NewManager(cfg ManagerConfig) *Manager {
	cfg.ApplyDefaults() // 自动填充默认值
	return &Manager{
		baseConfig: cfg,
		loggers:    make(map[string]*CtxZapLogger, cfg.ModuleNumber),
		zapLoggers: make(map[string]*zap.Logger, cfg.ModuleNumber),
		writers:    make(map[string][]*lumberjack.Logger, cfg.ModuleNumber),
	}
}

// InitManager 初始化全局 Logger 管理器（只调用一次）
func InitManager(cfg ManagerConfig) {
	managerOnce.Do(func() {
		globalManager = NewManager(cfg)
	})
}

func getSelfLogger() *CtxZapLogger {
	return GetLogger(globalManager.baseConfig.LoggerName)
}

// ============================================
// Manager 实例方法（核心实现）
// ============================================

// GetLogger 获取指定模块的 CtxZapLogger（线程安全，按需创建）
// 返回的 Logger 已自动包含 module 字段
func (m *Manager) GetLogger(moduleName string) *CtxZapLogger {
	// 先尝试读锁（快速路径）
	m.mu.RLock()
	if logger, exists := m.loggers[moduleName]; exists {
		m.mu.RUnlock()
		return logger
	}
	m.mu.RUnlock()

	// 不存在，创建新的 Logger（写锁）
	m.mu.Lock()
	defer m.mu.Unlock()

	// 双重检查（避免并发创建）
	if logger, exists := m.loggers[moduleName]; exists {
		return logger
	}

	// 创建该模块的配置
	cfg := m.buildModuleConfig(moduleName)

	// 创建底层 zap.Logger 实例
	zapLogger := m.createLogger(cfg)

	// 自动添加 module 字段
	zapLoggerWithModule := zapLogger.With(zap.String("module", moduleName))

	// 添加 CallerSkip，跳过 CtxZapLogger 的包装层
	zapLoggerWithSkip := zapLoggerWithModule.WithOptions(zap.AddCallerSkip(1))

	// 创建 CtxZapLogger 包装
	ctxLogger := &CtxZapLogger{
		base:   zapLoggerWithSkip,
		module: moduleName,
		config: &m.baseConfig,
	}

	// 缓存 CtxZapLogger 和底层 zap.Logger
	m.loggers[moduleName] = ctxLogger
	m.zapLoggers[moduleName] = zapLoggerWithModule

	return ctxLogger
}

// buildModuleConfig 为指定模块构建配置
func (m *Manager) buildModuleConfig(moduleName string) Config {
	return Config{
		Level:                    m.baseConfig.Level,
		Development:              false,
		Encoding:                 m.baseConfig.Encoding,
		ConsoleEncoding:          m.baseConfig.ConsoleEncoding,
		moduleName:               moduleName, // 内部字段：每个模块独立
		logDir:                   m.baseConfig.BaseLogDir,
		EnableFile:               true,
		EnableConsole:            m.baseConfig.EnableConsole,
		EnableLevelInFilename:    m.baseConfig.EnableLevelInFilename,
		EnableSequenceInFilename: m.baseConfig.EnableSequenceInFilename,
		SequenceNumber:           "",
		EnableDateInFilename:     m.baseConfig.EnableDateInFilename,
		DateFormat:               m.baseConfig.DateFormat,
		MaxSize:                  m.baseConfig.MaxSize,
		MaxBackups:               m.baseConfig.MaxBackups,
		MaxAge:                   m.baseConfig.MaxAge,
		Compress:                 m.baseConfig.Compress,
		EnableCaller:             m.baseConfig.EnableCaller,
		EnableStacktrace:         m.baseConfig.EnableStacktrace,
		StacktraceLevel:          m.baseConfig.StacktraceLevel,
	}
}

// createLogger 创建 Logger 实例
func (m *Manager) createLogger(cfg Config) *zap.Logger {
	encoder := createEncoder(cfg)
	var cores []zapcore.Core
	var writers []*lumberjack.Logger // 保存文件写入器引用

	// Console 输出
	if cfg.EnableConsole {
		consoleEncoder := encoder
		if cfg.ConsoleEncoding != "" && cfg.ConsoleEncoding != cfg.Encoding {
			cliCfg := cfg
			cliCfg.Encoding = cfg.ConsoleEncoding
			consoleEncoder = createEncoder(cliCfg)
		}
		consoleCore := zapcore.NewCore(
			consoleEncoder,
			zapcore.AddSync(os.Stdout),
			ParseLevel(cfg.Level),
		)
		cores = append(cores, consoleCore)
	}

	// 文件输出 - Info 级别
	if cfg.EnableFile {
		infoPath := cfg.getInfoFilePath()
		infoWriter, infoLumber := createFileWriter(infoPath, cfg)
		writers = append(writers, infoLumber) // 保存引用

		// 🎯 修复：根据配置的日志级别动态过滤
		// 如果配置级别是 info，只记录 info 和 warn（不包括 debug）
		// 如果配置级别是 debug，记录 debug、info 和 warn
		configuredLevel := ParseLevel(cfg.Level)
		infoCore := zapcore.NewCore(
			encoder,
			infoWriter,
			zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
				// 日志级别必须 >= 配置级别 且 < ErrorLevel
				return lvl >= configuredLevel && lvl < zapcore.ErrorLevel
			}),
		)
		cores = append(cores, infoCore)

		// 文件输出 - Error 级别
		errorPath := cfg.getErrorFilePath()
		errorWriter, errorLumber := createFileWriter(errorPath, cfg)
		writers = append(writers, errorLumber) // 保存引用
		errorCore := zapcore.NewCore(
			encoder,
			errorWriter,
			zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
				return lvl >= zapcore.ErrorLevel
			}),
		)
		cores = append(cores, errorCore)
	}

	core := zapcore.NewTee(cores...)

	// 添加选项
	opts := []zap.Option{}
	if cfg.EnableCaller {
		opts = append(opts, zap.AddCaller())
	}
	// 注意：不再使用 zap.AddStacktrace，改由 CtxZapLogger.ErrorCtx 自行控制堆栈深度
	// 这样可以精确控制堆栈层数，避免日志过长
	// if cfg.EnableStacktrace {
	// 	stackLevel := ParseLevel(cfg.StacktraceLevel)
	// 	opts = append(opts, zap.AddStacktrace(stackLevel))
	// }

	// 保存文件写入器引用（用于关闭）
	if len(writers) > 0 {
		m.writers[cfg.moduleName] = writers
	}

	return zap.New(core, opts...)
}

// CloseAll 关闭所有 Logger（应用退出时调用）
// 会刷新缓冲区并关闭所有文件句柄
func (m *Manager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 1. 刷新缓冲区
	for _, logger := range m.zapLoggers {
		_ = logger.Sync()
	}

	// 2. 关闭文件句柄
	for _, writers := range m.writers {
		for _, w := range writers {
			if err := w.Close(); err != nil {
				// 忽略错误，继续关闭其他文件
			}
		}
	}

	// 3. 清空 map
	m.loggers = make(map[string]*CtxZapLogger)
	m.zapLoggers = make(map[string]*zap.Logger)
	m.writers = make(map[string][]*lumberjack.Logger)
}

// ReloadConfig 热重载配置（重建所有 Logger 实例）
func (m *Manager) ReloadConfig(newCfg ManagerConfig) error {
	// 先验证新配置
	if err := newCfg.Validate(); err != nil {
		return fmt.Errorf("新配置验证失败: %w", err)
	}

	m.mu.Lock()

	// 保存旧配置（用于日志输出）
	oldLevel := m.baseConfig.Level
	oldEncoding := m.baseConfig.Encoding

	// 1. 刷新缓冲区
	for _, logger := range m.zapLoggers {
		_ = logger.Sync()
	}

	// 2. 关闭旧的文件句柄
	for _, writers := range m.writers {
		for _, w := range writers {
			_ = w.Close()
		}
	}

	// 3. 清空 map
	m.loggers = make(map[string]*CtxZapLogger)
	m.zapLoggers = make(map[string]*zap.Logger)
	m.writers = make(map[string][]*lumberjack.Logger)

	// 4. 更新基础配置
	m.baseConfig = newCfg

	m.mu.Unlock()

	// 释放锁后输出变更信息（避免死锁）
	if oldLevel != newCfg.Level {
		m.Debug("logger", "日志级别已更新",
			zap.String("old_level", oldLevel),
			zap.String("new_level", newCfg.Level))
	}

	if oldEncoding != newCfg.Encoding {
		m.Debug("logger", "日志编码已更新",
			zap.String("old_encoding", oldEncoding),
			zap.String("new_encoding", newCfg.Encoding))
	}

	return nil
}

// extractTraceID 从 context 提取 traceID
func (m *Manager) extractTraceID(ctx context.Context) string {
	if !m.baseConfig.EnableTraceID {
		return ""
	}

	key := m.baseConfig.TraceIDKey
	if key == "" {
		key = "trace_id"
	}

	if val := ctx.Value(key); val != nil {
		if traceID, ok := val.(string); ok {
			return traceID
		}
	}
	return ""
}

// buildFieldsWithTraceID 构建包含 traceID 的字段列表
func (m *Manager) buildFieldsWithTraceID(ctx context.Context, fields []zap.Field) []zap.Field {
	traceID := m.extractTraceID(ctx)
	if traceID == "" {
		return fields
	}

	fieldName := "trace_id"
	if m.baseConfig.TraceIDFieldName != "" {
		fieldName = m.baseConfig.TraceIDFieldName
	}

	// 将 traceID 放在最前面
	newFields := make([]zap.Field, 0, len(fields)+1)
	newFields = append(newFields, zap.String(fieldName, traceID))
	newFields = append(newFields, fields...)
	return newFields
}

// ============================================
// Manager 实例便捷方法
// ============================================

// Info 记录 Info 级别日志
func (m *Manager) Info(module string, msg string, fields ...zap.Field) {
	m.GetLogger(module).InfoCtx(context.Background(), msg, fields...)
}

// Debug 记录 Debug 级别日志
func (m *Manager) Debug(module string, msg string, fields ...zap.Field) {
	m.GetLogger(module).DebugCtx(context.Background(), msg, fields...)
}

// Warn 记录 Warn 级别日志
func (m *Manager) Warn(module string, msg string, fields ...zap.Field) {
	m.GetLogger(module).WarnCtx(context.Background(), msg, fields...)
}

// Error 记录 Error 级别日志
func (m *Manager) Error(module string, msg string, fields ...zap.Field) {
	m.GetLogger(module).ErrorCtx(context.Background(), msg, fields...)
}

// Fatal 记录 Fatal 级别日志（会调用 os.Exit(1)）
func (m *Manager) Fatal(module string, msg string, fields ...zap.Field) {
	m.GetLogger(module).GetZapLogger().Fatal(msg, fields...)
}

// Panic 记录 Panic 级别日志（会触发 panic）
func (m *Manager) Panic(module string, msg string, fields ...zap.Field) {
	m.GetLogger(module).GetZapLogger().Panic(msg, fields...)
}

// WithFields 为指定模块创建带预设字段的 Logger
func (m *Manager) WithFields(module string, fields ...zap.Field) *CtxZapLogger {
	return m.GetLogger(module).With(fields...)
}

// InfoCtx 记录 Info 级别日志（支持从 context 提取 traceID）
func (m *Manager) InfoCtx(ctx context.Context, module string, msg string, fields ...zap.Field) {
	m.GetLogger(module).InfoCtx(ctx, msg, fields...)
}

// DebugCtx 记录 Debug 级别日志（支持从 context 提取 traceID）
func (m *Manager) DebugCtx(ctx context.Context, module string, msg string, fields ...zap.Field) {
	m.GetLogger(module).DebugCtx(ctx, msg, fields...)
}

// WarnCtx 记录 Warn 级别日志（支持从 context 提取 traceID）
func (m *Manager) WarnCtx(ctx context.Context, module string, msg string, fields ...zap.Field) {
	m.GetLogger(module).WarnCtx(ctx, msg, fields...)
}

// ErrorCtx 记录 Error 级别日志（支持从 context 提取 traceID）
func (m *Manager) ErrorCtx(ctx context.Context, module string, msg string, fields ...zap.Field) {
	m.GetLogger(module).ErrorCtx(ctx, msg, fields...)
}

// FatalCtx 记录 Fatal 级别日志（会调用 os.Exit(1)，支持从 context 提取 traceID）
func (m *Manager) FatalCtx(ctx context.Context, module string, msg string, fields ...zap.Field) {
	fields = m.buildFieldsWithTraceID(ctx, fields)
	m.GetLogger(module).GetZapLogger().Fatal(msg, fields...)
}

// PanicCtx 记录 Panic 级别日志（会触发 panic，支持从 context 提取 traceID）
func (m *Manager) PanicCtx(ctx context.Context, module string, msg string, fields ...zap.Field) {
	fields = m.buildFieldsWithTraceID(ctx, fields)
	m.GetLogger(module).GetZapLogger().Panic(msg, fields...)
}

// ============================================
// 全局辅助函数（非导出）
// ============================================

// createEncoder 创建编码器
func createEncoder(cfg Config) zapcore.Encoder {
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		MessageKey:     "msg",
		CallerKey:      "caller",
		StacktraceKey:  "stack",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	switch cfg.Encoding {
	case "console":
		return zapcore.NewConsoleEncoder(encoderConfig)
	case "console_pretty":
		// 使用渲染样式创建编码器
		style := ParseRenderStyle(globalManager.baseConfig.RenderStyle)
		return NewPrettyConsoleEncoderWithStyle(encoderConfig, style)
	default:
		return zapcore.NewJSONEncoder(encoderConfig)
	}
}

// createFileWriter 创建文件写入器（支持切割）
// 返回 WriteSyncer 和 lumberjack.Logger（用于关闭文件句柄）
func createFileWriter(filename string, cfg Config) (zapcore.WriteSyncer, *lumberjack.Logger) {
	// 确保目录存在
	dir := filepath.Dir(filename)
	os.MkdirAll(dir, 0755)

	// 使用 lumberjack 实现文件切割
	lumberLogger := &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    cfg.MaxSize,    // MB
		MaxBackups: cfg.MaxBackups, // 保留数量
		MaxAge:     cfg.MaxAge,     // 保留天数
		Compress:   cfg.Compress,   // 是否压缩
		LocalTime:  true,
	}

	return zapcore.AddSync(lumberLogger), lumberLogger
}

// ============================================
// 包级别便捷函数（调用 globalManager，保持向后兼容）
// ============================================

// GetLogger 获取指定模块的 CtxZapLogger（线程安全，按需创建）
func GetLogger(moduleName string) *CtxZapLogger {
	if globalManager == nil {
		// 如果没有初始化，使用默认配置
		InitManager(DefaultManagerConfig())
	}
	return globalManager.GetLogger(moduleName)
}

// CloseAll 关闭所有 Logger（应用退出时调用）
func CloseAll() {
	if globalManager == nil {
		return
	}
	globalManager.CloseAll()
}

// ReloadConfig 热重载配置（重建所有 Logger 实例）
func ReloadConfig(newCfg ManagerConfig) error {
	if globalManager == nil {
		return fmt.Errorf("Logger 管理器未初始化")
	}
	return globalManager.ReloadConfig(newCfg)
}

// Info 记录 Info 级别日志
// 用法：logger.Info("order", "Order creation", zap.String("id", "001"))
// 生成：logs/order/order-info-2024-12-19.log
func Info(module string, msg string, fields ...zap.Field) {
	if globalManager == nil {
		InitManager(DefaultManagerConfig())
	}
	globalManager.Info(module, msg, fields...)
}

// Debug 记录 Debug 级别日志
func Debug(module string, msg string, fields ...zap.Field) {
	if globalManager == nil {
		InitManager(DefaultManagerConfig())
	}
	globalManager.Debug(module, msg, fields...)
}

// Warn 记录 Warn 级别日志
func Warn(module string, msg string, fields ...zap.Field) {
	if globalManager == nil {
		InitManager(DefaultManagerConfig())
	}
	globalManager.Warn(module, msg, fields...)
}

// Error 记录 Error 级别日志
// 用法：logger.Error("auth", "Login failed", zap.String("user", "admin"))
// 生成：logs/auth/auth-error-2024-12-19.log
func Error(module string, msg string, fields ...zap.Field) {
	if globalManager == nil {
		InitManager(DefaultManagerConfig())
	}
	globalManager.Error(module, msg, fields...)
}

// Fatal 记录 Fatal 级别日志（会调用 os.Exit(1)）
func Fatal(module string, msg string, fields ...zap.Field) {
	if globalManager == nil {
		InitManager(DefaultManagerConfig())
	}
	globalManager.Fatal(module, msg, fields...)
}

// Panic 记录 Panic 级别日志（会触发 panic）
func Panic(module string, msg string, fields ...zap.Field) {
	if globalManager == nil {
		InitManager(DefaultManagerConfig())
	}
	globalManager.Panic(module, msg, fields...)
}

// WithFields 为指定模块创建带预设字段的 Logger
// 用法：
//
//	orderLogger := logger.WithFields("order", zap.String("service", "order-service"))
//	orderLogger.InfoCtx(ctx, "Order creation")  // 自动包含 service 字段
func WithFields(module string, fields ...zap.Field) *CtxZapLogger {
	if globalManager == nil {
		InitManager(DefaultManagerConfig())
	}
	return globalManager.WithFields(module, fields...)
}

// InfoCtx 记录 Info 级别日志（支持从 context 提取 traceID）
// 用法：logger.InfoCtx(ctx, "order", "Order creation", zap.String("id", "001"))
// 如果 ctx 中包含 traceID，会自动添加到日志字段中
func InfoCtx(ctx context.Context, module string, msg string, fields ...zap.Field) {
	if globalManager == nil {
		InitManager(DefaultManagerConfig())
	}
	globalManager.InfoCtx(ctx, module, msg, fields...)
}

// DebugCtx 记录 Debug 级别日志（支持从 context 提取 traceID）
func DebugCtx(ctx context.Context, module string, msg string, fields ...zap.Field) {
	if globalManager == nil {
		InitManager(DefaultManagerConfig())
	}
	globalManager.DebugCtx(ctx, module, msg, fields...)
}

// WarnCtx 记录 Warn 级别日志（支持从 context 提取 traceID）
func WarnCtx(ctx context.Context, module string, msg string, fields ...zap.Field) {
	if globalManager == nil {
		InitManager(DefaultManagerConfig())
	}
	globalManager.WarnCtx(ctx, module, msg, fields...)
}

// ErrorCtx 记录 Error 级别日志（支持从 context 提取 traceID）
// 用法：logger.ErrorCtx(ctx, "auth", "Login failed", zap.String("user", "admin"))
// 如果 ctx 中包含 traceID，会自动添加到日志字段中
func ErrorCtx(ctx context.Context, module string, msg string, fields ...zap.Field) {
	if globalManager == nil {
		InitManager(DefaultManagerConfig())
	}
	globalManager.ErrorCtx(ctx, module, msg, fields...)
}

// FatalCtx 记录 Fatal 级别日志（会调用 os.Exit(1)，支持从 context 提取 traceID）
func FatalCtx(ctx context.Context, module string, msg string, fields ...zap.Field) {
	if globalManager == nil {
		InitManager(DefaultManagerConfig())
	}
	globalManager.FatalCtx(ctx, module, msg, fields...)
}

// PanicCtx 记录 Panic 级别日志（会触发 panic，支持从 context 提取 traceID）
func PanicCtx(ctx context.Context, module string, msg string, fields ...zap.Field) {
	if globalManager == nil {
		InitManager(DefaultManagerConfig())
	}
	globalManager.PanicCtx(ctx, module, msg, fields...)
}
