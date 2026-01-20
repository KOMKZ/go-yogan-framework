package logger

import (
	"strings"
)

// GinLogWriter Gin log adapter (implements the io.Writer interface)
// Adapt Gin's text logs to a custom Logger component
type GinLogWriter struct {
	module string // Log module name (e.g., gin-route, gin-internal)
}

// Create Gin log adapter
// module: log module name, used to distinguish logs from different sources of Gin
// "gin-route": routing registration logs
// "gin-internal": other kernel logs
func NewGinLogWriter(module string) *GinLogWriter {
	return &GinLogWriter{module: module}
}

// Implement the io.Writer interface
// The Gin framework will call this method to write logs
// Convert Gin's text logs to structured log output
func (w *GinLogWriter) Write(p []byte) (n int, err error) {
	// Remove leading and trailing spaces and newline characters
	msg := strings.TrimSpace(string(p))
	if msg == "" {
		return len(p), nil
	}

	// Handle logging based on content categories
	if strings.Contains(msg, "[GIN-debug]") {
		// Route registration log: Use Debug level
		Debug(w.module, msg)
	} else if strings.Contains(msg, "[GIN]") {
		// HTTP request logs (if using the gin.Logger middleware): use Info level
		Info(w.module, msg)
	} else if strings.Contains(msg, "[Recovery]") || strings.Contains(msg, "panic recovered") {
		// Recovery log: Use Error level
		Error(w.module, msg)
	} else {
		// Other logs: use Info level
		Info(w.module, msg)
	}

	return len(p), nil
}

