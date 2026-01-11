// src/pkg/logger/ctx_zap_logger_testing.go
package logger

import (
	"context"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// TestCtxLogger 测试专用的 Context-Aware Logger
// 将日志记录到内存，方便在单元测试中验证
type TestCtxLogger struct {
	logs []LogEntry
	mu   sync.RWMutex
}

// LogEntry 日志条目
type LogEntry struct {
	Level   string
	Message string
	TraceID string
	Fields  map[string]interface{}
}

// NewTestCtxLogger 创建测试用 Logger（记录到内存）
// 用法：
//
//	testLogger := logger.NewTestCtxLogger()
//	svc := user.NewService(repo, testLogger)
//	svc.CreateUser(ctx, "test", "test@example.com", 25)
//	assert.True(t, testLogger.HasLog("INFO", "Create user"))
func NewTestCtxLogger() *TestCtxLogger {
	return &TestCtxLogger{
		logs: make([]LogEntry, 0),
	}
}

// InfoCtx 记录 Info 级别日志（记录到内存）
func (t *TestCtxLogger) InfoCtx(ctx context.Context, msg string, fields ...zap.Field) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.logs = append(t.logs, LogEntry{
		Level:   "INFO",
		Message: msg,
		TraceID: extractTraceIDFromContext(ctx, nil),
		Fields:  extractFieldsMap(fields),
	})
}

// ErrorCtx 记录 Error 级别日志（记录到内存）
func (t *TestCtxLogger) ErrorCtx(ctx context.Context, msg string, fields ...zap.Field) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.logs = append(t.logs, LogEntry{
		Level:   "ERROR",
		Message: msg,
		TraceID: extractTraceIDFromContext(ctx, nil),
		Fields:  extractFieldsMap(fields),
	})
}

// DebugCtx 记录 Debug 级别日志（记录到内存）
func (t *TestCtxLogger) DebugCtx(ctx context.Context, msg string, fields ...zap.Field) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.logs = append(t.logs, LogEntry{
		Level:   "DEBUG",
		Message: msg,
		TraceID: extractTraceIDFromContext(ctx, nil),
		Fields:  extractFieldsMap(fields),
	})
}

// WarnCtx 记录 Warn 级别日志（记录到内存）
func (t *TestCtxLogger) WarnCtx(ctx context.Context, msg string, fields ...zap.Field) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.logs = append(t.logs, LogEntry{
		Level:   "WARN",
		Message: msg,
		TraceID: extractTraceIDFromContext(ctx, nil),
		Fields:  extractFieldsMap(fields),
	})
}

// With 返回带有预设字段的新 Logger（用于测试）
func (t *TestCtxLogger) With(fields ...zap.Field) *TestCtxLogger {
	// 测试版本：返回新实例，共享日志存储
	newLogger := &TestCtxLogger{
		logs: t.logs, // 共享 logs（用于验证）
		mu:   t.mu,   // 共享锁
	}
	return newLogger
}

// ============================================
// 断言辅助方法
// ============================================

// HasLog 检查是否存在指定级别和消息的日志
func (t *TestCtxLogger) HasLog(level, message string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for _, log := range t.logs {
		if log.Level == level && log.Message == message {
			return true
		}
	}
	return false
}

// HasLogWithTraceID 检查是否存在指定级别、消息和 TraceID 的日志
func (t *TestCtxLogger) HasLogWithTraceID(level, message, traceID string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for _, log := range t.logs {
		if log.Level == level && log.Message == message && log.TraceID == traceID {
			return true
		}
	}
	return false
}

// HasLogWithField 检查是否存在指定级别、消息和字段的日志
func (t *TestCtxLogger) HasLogWithField(level, message, fieldKey string, fieldValue interface{}) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for _, log := range t.logs {
		if log.Level == level && log.Message == message {
			if val, exists := log.Fields[fieldKey]; exists && val == fieldValue {
				return true
			}
		}
	}
	return false
}

// CountLogs 统计指定级别的日志数量
func (t *TestCtxLogger) CountLogs(level string) int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	count := 0
	for _, log := range t.logs {
		if log.Level == level {
			count++
		}
	}
	return count
}

// Logs 获取所有日志（用于详细验证）
func (t *TestCtxLogger) Logs() []LogEntry {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// 返回副本，避免外部修改
	logs := make([]LogEntry, len(t.logs))
	copy(logs, t.logs)
	return logs
}

// Clear 清空日志（用于测试隔离）
func (t *TestCtxLogger) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.logs = make([]LogEntry, 0)
}

// ============================================
// 内部辅助函数
// ============================================

// extractFieldsMap 将 zap.Field 转换为 map（用于测试断言）
func extractFieldsMap(fields []zap.Field) map[string]interface{} {
	result := make(map[string]interface{}, len(fields))
	
	// 使用 zap.NewProductionEncoderConfig 临时编码字段
	enc := zapcore.NewMapObjectEncoder()
	
	for _, field := range fields {
		field.AddTo(enc)
	}
	
	// 将编码器的结果转换为 map
	for k, v := range enc.Fields {
		result[k] = v
	}
	
	return result
}

