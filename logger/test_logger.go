// src/pkg/logger/ctx_zap_logger_testing.go
package logger

import (
	"context"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// TestCtxLogger context-aware logger for testing purposes
// Log to memory for convenient verification in unit tests
type TestCtxLogger struct {
	logs []LogEntry
	mu   sync.RWMutex
}

// Log Entry
type LogEntry struct {
	Level   string
	Message string
	TraceID string
	Fields  map[string]interface{}
}

// Create test Logger (log to memory)
// Usage:
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

// InfoCtx logs info level logs (recorded in memory)
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

// ErrorCtx logs error level logs (records to memory)
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

// DebugCtx logs debug level logs (records to memory)
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

// WarnCtx logs Warn level messages (stored in memory)
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

// Returns a new Logger with preset fields (for testing)
func (t *TestCtxLogger) With(fields ...zap.Field) *TestCtxLogger {
	// Test version: return new instance, shared log storage
	newLogger := &TestCtxLogger{
		logs: t.logs, // Shared logs (for verification)
		mu:   t.mu,   // shared lock
	}
	return newLogger
}

// ============================================
// Assertion helper method
// ============================================

// Check if a log with the specified level and message exists
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

// Checks if a log with specified level, message, and TraceID exists
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

// Checks if a log with specified level, message, and field exists
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

// CountLogs counts the number of logs at a specified level
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

// Logs Retrieve all logs (for detailed verification)
func (t *TestCtxLogger) Logs() []LogEntry {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// Return a copy to avoid external modification
	logs := make([]LogEntry, len(t.logs))
	copy(logs, t.logs)
	return logs
}

// Clear logs (for test isolation)
func (t *TestCtxLogger) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.logs = make([]LogEntry, 0)
}

// ============================================
// Internal auxiliary function
// ============================================

// extractFieldsMap converts zap.Field to a map (for test assertions)
func extractFieldsMap(fields []zap.Field) map[string]interface{} {
	result := make(map[string]interface{}, len(fields))
	
	// Use zap.NewProductionEncoderConfig to temporarily encode fields
	enc := zapcore.NewMapObjectEncoder()
	
	for _, field := range fields {
		field.AddTo(enc)
	}
	
	// Convert the encoder's result to a map
	for k, v := range enc.Fields {
		result[k] = v
	}
	
	return result
}

