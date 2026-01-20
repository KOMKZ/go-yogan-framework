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

// Manager Logger (manages multiple Logger instances)
type Manager struct {
	baseConfig ManagerConfig
	loggers    map[string]*CtxZapLogger        // Module name -> CtxZapLogger instance
	zapLoggers map[string]*zap.Logger          // Module name -> underlying zap.Logger instance
	writers    map[string][]*lumberjack.Logger // Module name -> File writer (for closing)
	mu         sync.RWMutex                    // concurrent safety
}

var (
	globalManager *Manager
	managerOnce   sync.Once
)

// NewManager creates independent Manager instances (supports multi-instance scenarios)
// Usage:
//
//	appManager := logger.NewManager(cfg)
//	appManager.Info("order", "Order creation")
//
// NewManager creates independent Manager instances
// zero-valued fields in cfg will be automatically filled with default values
func NewManager(cfg ManagerConfig) *Manager {
	cfg.ApplyDefaults() // Auto-fill default values
	return &Manager{
		baseConfig: cfg,
		loggers:    make(map[string]*CtxZapLogger, cfg.ModuleNumber),
		zapLoggers: make(map[string]*zap.Logger, cfg.ModuleNumber),
		writers:    make(map[string][]*lumberjack.Logger, cfg.ModuleNumber),
	}
}

// Initialize Manager for global Logger manager (call once only)
func InitManager(cfg ManagerConfig) {
	managerOnce.Do(func() {
		globalManager = NewManager(cfg)
	})
}

func MustResetManager(cfg ManagerConfig) {
	globalManager = NewManager(cfg)
}

func getSelfLogger() *CtxZapLogger {
	return GetLogger(globalManager.baseConfig.LoggerName)
}

// ============================================
// Manager instance method (core implementation)
// ============================================

// GetLogger obtain a thread-safe CtxZapLogger for the specified module (created as needed)
// The returned Logger automatically includes a module field
func (m *Manager) GetLogger(moduleName string) *CtxZapLogger {
	// Try read lock first (fast path)
	m.mu.RLock()
	if logger, exists := m.loggers[moduleName]; exists {
		m.mu.RUnlock()
		return logger
	}
	m.mu.RUnlock()

	// Does not exist, create new Logger (write lock)
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double check (avoid concurrent creation)
	if logger, exists := m.loggers[moduleName]; exists {
		return logger
	}

	// Create the module's configuration
	cfg := m.buildModuleConfig(moduleName)

	// Create a zap.Logger instance
	zapLogger := m.createLogger(cfg)

	// Automatically add module field
	zapLoggerWithModule := zapLogger.With(zap.String("module", moduleName))

	// Add CallerSkip to skip the CtxZapLogger wrapper layer
	zapLoggerWithSkip := zapLoggerWithModule.WithOptions(zap.AddCallerSkip(1))

	// Create CtxZapLogger wrapper
	ctxLogger := &CtxZapLogger{
		base:   zapLoggerWithSkip,
		module: moduleName,
		config: &m.baseConfig,
	}

	// Cache CtxZapLogger and the underlying zap.Logger
	m.loggers[moduleName] = ctxLogger
	m.zapLoggers[moduleName] = zapLoggerWithModule

	return ctxLogger
}

// buildModuleConfig builds configuration for specified module
func (m *Manager) buildModuleConfig(moduleName string) Config {
	return Config{
		Level:                    m.baseConfig.Level,
		Development:              false,
		Encoding:                 m.baseConfig.Encoding,
		ConsoleEncoding:          m.baseConfig.ConsoleEncoding,
		moduleName:               moduleName, // Internal fields: Each module is independent
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

// createLogger Create Logger instance
func (m *Manager) createLogger(cfg Config) *zap.Logger {
	encoder := createEncoder(cfg)
	var cores []zapcore.Core
	var writers []*lumberjack.Logger // Save file writer reference

	// Console output
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

	// File output - Info level
	if cfg.EnableFile {
		infoPath := cfg.getInfoFilePath()
		infoWriter, infoLumber := createFileWriter(infoPath, cfg)
		writers = append(writers, infoLumber) // save reference

		// TARGET: Fix: Dynamically filter based on configured log level
		// If the configuration level is info, only record info and warn (excluding debug)
		// If the configuration level is debug, log debug, info, and warn
		configuredLevel := ParseLevel(cfg.Level)
		infoCore := zapcore.NewCore(
			encoder,
			infoWriter,
			zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
				// Log level must be >= configuration level AND < ErrorLevel
				return lvl >= configuredLevel && lvl < zapcore.ErrorLevel
			}),
		)
		cores = append(cores, infoCore)

		// File output - Error level
		errorPath := cfg.getErrorFilePath()
		errorWriter, errorLumber := createFileWriter(errorPath, cfg)
		writers = append(writers, errorLumber) // save reference
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

	// Add option
	opts := []zap.Option{}
	if cfg.EnableCaller {
		opts = append(opts, zap.AddCaller())
	}
	// Note: Stop using zap.AddStacktrace, use CtxZapLogger.ErrorCtx to control stack depth manually
	// This allows precise control over the stack depth, avoiding excessively long logs
	// if cfg.EnableStacktrace {
	// 	stackLevel := ParseLevel(cfg.StacktraceLevel)
	// 	opts = append(opts, zap.AddStacktrace(stackLevel))
	// }

	// Save file writer reference (for closing)
	if len(writers) > 0 {
		m.writers[cfg.moduleName] = writers
	}

	return zap.New(core, opts...)
}

// CloseAll closes all Loggers (called when the application exits)
// will refresh the buffer and close all file handles
func (m *Manager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Refresh buffer
	for _, logger := range m.zapLoggers {
		_ = logger.Sync()
	}

	// Close file handle
	for _, writers := range m.writers {
		for _, w := range writers {
			if err := w.Close(); err != nil {
				// Ignore errors, continue closing other files
			}
		}
	}

	// 3. Clear map
	m.loggers = make(map[string]*CtxZapLogger)
	m.zapLoggers = make(map[string]*zap.Logger)
	m.writers = make(map[string][]*lumberjack.Logger)
}

// ReloadConfig hot reload configuration (recreate all Logger instances)
func (m *Manager) ReloadConfig(newCfg ManagerConfig) error {
	// Validate new configuration first
	if err := newCfg.Validate(); err != nil {
		return fmt.Errorf("Configuration validation failed: %w: %w", err)
	}

	m.mu.Lock()

	// Save old configuration (for log output)
	oldLevel := m.baseConfig.Level
	oldEncoding := m.baseConfig.Encoding

	// Refresh buffer
	for _, logger := range m.zapLoggers {
		_ = logger.Sync()
	}

	// Close old file handle
	for _, writers := range m.writers {
		for _, w := range writers {
			_ = w.Close()
		}
	}

	// 3. Clear map
	m.loggers = make(map[string]*CtxZapLogger)
	m.zapLoggers = make(map[string]*zap.Logger)
	m.writers = make(map[string][]*lumberjack.Logger)

	// 4. Update basic configuration
	m.baseConfig = newCfg

	m.mu.Unlock()

	// Release the lock before outputting the change information (to avoid deadlocks)
	if oldLevel != newCfg.Level {
		m.Debug("logger", "English: Log level has been updated",
			zap.String("old_level", oldLevel),
			zap.String("new_level", newCfg.Level))
	}

	if oldEncoding != newCfg.Encoding {
		m.Debug("logger", "Log encoding has been updated",
			zap.String("old_encoding", oldEncoding),
			zap.String("new_encoding", newCfg.Encoding))
	}

	return nil
}

// extractTraceID extract traceID from context
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

// buildFieldsWithTraceID Build a list of fields including traceID
func (m *Manager) buildFieldsWithTraceID(ctx context.Context, fields []zap.Field) []zap.Field {
	traceID := m.extractTraceID(ctx)
	if traceID == "" {
		return fields
	}

	fieldName := "trace_id"
	if m.baseConfig.TraceIDFieldName != "" {
		fieldName = m.baseConfig.TraceIDFieldName
	}

	// Put traceID at the very front
	newFields := make([]zap.Field, 0, len(fields)+1)
	newFields = append(newFields, zap.String(fieldName, traceID))
	newFields = append(newFields, fields...)
	return newFields
}

// ============================================
// Convenient methods for Manager instance
// ============================================

// Info log for Info level logging
func (m *Manager) Info(module string, msg string, fields ...zap.Field) {
	m.GetLogger(module).InfoCtx(context.Background(), msg, fields...)
}

// Debug logging for Debug level logs
func (m *Manager) Debug(module string, msg string, fields ...zap.Field) {
	m.GetLogger(module).DebugCtx(context.Background(), msg, fields...)
}

// Warn Record Warn level log
func (m *Manager) Warn(module string, msg string, fields ...zap.Field) {
	m.GetLogger(module).WarnCtx(context.Background(), msg, fields...)
}

// Record error level logs
func (m *Manager) Error(module string, msg string, fields ...zap.Field) {
	m.GetLogger(module).ErrorCtx(context.Background(), msg, fields...)
}

// Fatal logs are recorded at the fatal level (will call os.Exit(1))
func (m *Manager) Fatal(module string, msg string, fields ...zap.Field) {
	m.GetLogger(module).GetZapLogger().Fatal(msg, fields...)
}

// Panic log at panic level (will trigger a panic)
func (m *Manager) Panic(module string, msg string, fields ...zap.Field) {
	m.GetLogger(module).GetZapLogger().Panic(msg, fields...)
}

// WithFields creates a Logger for the specified module with preset fields
func (m *Manager) WithFields(module string, fields ...zap.Field) *CtxZapLogger {
	return m.GetLogger(module).With(fields...)
}

// InfoCtx logs info level logs (supports extracting traceID from context)
func (m *Manager) InfoCtx(ctx context.Context, module string, msg string, fields ...zap.Field) {
	m.GetLogger(module).InfoCtx(ctx, msg, fields...)
}

// DebugCtx logs debug level logs (supports extracting traceID from context)
func (m *Manager) DebugCtx(ctx context.Context, module string, msg string, fields ...zap.Field) {
	m.GetLogger(module).DebugCtx(ctx, msg, fields...)
}

// WarnCtx logs warnings (supports extracting traceID from context)
func (m *Manager) WarnCtx(ctx context.Context, module string, msg string, fields ...zap.Field) {
	m.GetLogger(module).WarnCtx(ctx, msg, fields...)
}

// ErrorCtx logs error level messages (supports extracting traceID from context)
func (m *Manager) ErrorCtx(ctx context.Context, module string, msg string, fields ...zap.Field) {
	m.GetLogger(module).ErrorCtx(ctx, msg, fields...)
}

// FatalCtx logs at the Fatal level (calls os.Exit(1)) and supports extracting traceID from context
func (m *Manager) FatalCtx(ctx context.Context, module string, msg string, fields ...zap.Field) {
	fields = m.buildFieldsWithTraceID(ctx, fields)
	m.GetLogger(module).GetZapLogger().Fatal(msg, fields...)
}

// PanicCtx logs at the Panic level (triggers a panic and supports extracting traceID from context)
func (m *Manager) PanicCtx(ctx context.Context, module string, msg string, fields ...zap.Field) {
	fields = m.buildFieldsWithTraceID(ctx, fields)
	m.GetLogger(module).GetZapLogger().Panic(msg, fields...)
}

// ============================================
// Global helper functions (not exported)
// ============================================

// createEncoder Create encoder
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
		// Use rendering style to create encoder
		style := ParseRenderStyle(globalManager.baseConfig.RenderStyle)
		return NewPrettyConsoleEncoderWithStyle(encoderConfig, style)
	default:
		return zapcore.NewJSONEncoder(encoderConfig)
	}
}

// createFileWriter Create file writer (supports splitting)
// Return WriteSyncer and lumberjack.Logger (for closing file handles)
func createFileWriter(filename string, cfg Config) (zapcore.WriteSyncer, *lumberjack.Logger) {
	// Ensure directory exists
	dir := filepath.Dir(filename)
	os.MkdirAll(dir, 0755)

	// Use lumberjack to implement file slicing
	lumberLogger := &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    cfg.MaxSize,    // MB
		MaxBackups: cfg.MaxBackups, // reserved quantity
		MaxAge:     cfg.MaxAge,     // Number of days to retain
		Compress:   cfg.Compress,   // Whether to compress
		LocalTime:  true,
	}

	return zapcore.AddSync(lumberLogger), lumberLogger
}

// ============================================
// package-level convenience functions (call globalManager, maintain backward compatibility)
// ============================================

// GetLogger obtain the CtxZapLogger for the specified module (thread-safe, created as needed)
func GetLogger(moduleName string) *CtxZapLogger {
	if globalManager == nil {
		// If not initialized, use default configuration
		InitManager(DefaultManagerConfig())
	}
	return globalManager.GetLogger(moduleName)
}

// CloseAll closes all Loggers (called when the application exits)
func CloseAll() {
	if globalManager == nil {
		return
	}
	globalManager.CloseAll()
}

// ReloadConfig Hot reload configuration (recreate all Logger instances)
func ReloadConfig(newCfg ManagerConfig) error {
	if globalManager == nil {
		return fmt.Errorf("Logger Logger manager not initialized")
	}
	return globalManager.ReloadConfig(newCfg)
}

// Info log for Info level logging
// Usage: logger.Info("order", "Order creation", zap.String("id", "001"))
// Generate: logs/order/order-info-2024-12-19.log
func Info(module string, msg string, fields ...zap.Field) {
	if globalManager == nil {
		InitManager(DefaultManagerConfig())
	}
	globalManager.Info(module, msg, fields...)
}

// Debug logging for DEBUG level logs
func Debug(module string, msg string, fields ...zap.Field) {
	if globalManager == nil {
		InitManager(DefaultManagerConfig())
	}
	globalManager.Debug(module, msg, fields...)
}

// Warn record Warn level log
func Warn(module string, msg string, fields ...zap.Field) {
	if globalManager == nil {
		InitManager(DefaultManagerConfig())
	}
	globalManager.Warn(module, msg, fields...)
}

// Record error level logs
// Usage: logger.Error("auth", "Login failed", zap.String("user", "admin"))
// Generate: logs/auth/auth-error-2024-12-19.log
func Error(module string, msg string, fields ...zap.Field) {
	if globalManager == nil {
		InitManager(DefaultManagerConfig())
	}
	globalManager.Error(module, msg, fields...)
}

// Fatal log at Fatal level (will call os.Exit(1))
func Fatal(module string, msg string, fields ...zap.Field) {
	if globalManager == nil {
		InitManager(DefaultManagerConfig())
	}
	globalManager.Fatal(module, msg, fields...)
}

// Panic log at panic level (triggers a panic)
func Panic(module string, msg string, fields ...zap.Field) {
	if globalManager == nil {
		InitManager(DefaultManagerConfig())
	}
	globalManager.Panic(module, msg, fields...)
}

// WithFields creates a Logger for the specified module with preset fields
// Usage:
//
//	orderLogger := logger.WithFields("order", zap.String("service", "order-service"))
// orderLogger.InfoCtx(ctx, "Order creation")  // automatically includes service field
func WithFields(module string, fields ...zap.Field) *CtxZapLogger {
	if globalManager == nil {
		InitManager(DefaultManagerConfig())
	}
	return globalManager.WithFields(module, fields...)
}

// InfoCtx logs information level logs (supports extracting traceID from context)
// Usage: logger.InfoCtx(ctx, "order", "Order creation", zap.String("id", "001"))
// If ctx contains a traceID, it will automatically be added to the log fields.
func InfoCtx(ctx context.Context, module string, msg string, fields ...zap.Field) {
	if globalManager == nil {
		InitManager(DefaultManagerConfig())
	}
	globalManager.InfoCtx(ctx, module, msg, fields...)
}

// DebugCtx logs debug level logs (supports extracting traceID from context)
func DebugCtx(ctx context.Context, module string, msg string, fields ...zap.Field) {
	if globalManager == nil {
		InitManager(DefaultManagerConfig())
	}
	globalManager.DebugCtx(ctx, module, msg, fields...)
}

// WarnCtx logs warnings (supports extracting traceID from context)
func WarnCtx(ctx context.Context, module string, msg string, fields ...zap.Field) {
	if globalManager == nil {
		InitManager(DefaultManagerConfig())
	}
	globalManager.WarnCtx(ctx, module, msg, fields...)
}

// ErrorCtx logs error level messages (supports extracting traceID from context)
// Usage: logger.ErrorCtx(ctx, "auth", "Login failed", zap.String("user", "admin"))
// If ctx contains a traceID, it will be automatically added to the log fields.
func ErrorCtx(ctx context.Context, module string, msg string, fields ...zap.Field) {
	if globalManager == nil {
		InitManager(DefaultManagerConfig())
	}
	globalManager.ErrorCtx(ctx, module, msg, fields...)
}

// FatalCtx logs at the Fatal level (calls os.Exit(1)) and supports extracting traceID from context
func FatalCtx(ctx context.Context, module string, msg string, fields ...zap.Field) {
	if globalManager == nil {
		InitManager(DefaultManagerConfig())
	}
	globalManager.FatalCtx(ctx, module, msg, fields...)
}

// PanicCtx logs panic level logs (triggers a panic, supports extracting traceID from context)
func PanicCtx(ctx context.Context, module string, msg string, fields ...zap.Field) {
	if globalManager == nil {
		InitManager(DefaultManagerConfig())
	}
	globalManager.PanicCtx(ctx, module, msg, fields...)
}
