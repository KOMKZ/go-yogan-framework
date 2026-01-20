// src/yogan/logger/stacktrace.go
package logger

import (
	"fmt"
	"runtime"
	"strings"
)

// CaptureStackTrace captures the current call stack (with depth limit support)
// skip: number of stack frames to skip (usually 2-3, skipping CaptureStacktrace and the caller itself)
// depth: maximum depth (0 means unlimited, recommended 5-10)
// Return the formatted stack string, one call frame per line
func CaptureStacktrace(skip int, depth int) string {
	maxDepth := depth
	if maxDepth <= 0 {
		maxDepth = 32 // Default maximum 32 layers
	}

	var frames []string
	// reserve more space to ensure sufficient frames are captured
	pcs := make([]uintptr, maxDepth*2)
	n := runtime.Callers(skip, pcs)

	if n == 0 {
		return ""
	}

	callersFrames := runtime.CallersFrames(pcs[:n])
	frameCount := 0
	for {
		frame, more := callersFrames.Next()

		// Format: function name
		// File: Line number
		frames = append(frames, fmt.Sprintf("%s\n\t%s:%d", frame.Function, frame.File, frame.Line))
		frameCount++

		// reached depth limit or no more frames
		if frameCount >= maxDepth || !more {
			break
		}
	}

	return strings.Join(frames, "\n")
}

// shouldCaptureStacktrace determines whether the current log level requires capturing a stack trace
func shouldCaptureStacktrace(level string, config ManagerConfig) bool {
	if !config.EnableStacktrace {
		return false
	}

	// Define level priority
	levels := map[string]int{
		"debug": 0,
		"info":  1,
		"warn":  2,
		"error": 3,
		"fatal": 4,
	}

	currentLevel := levels[level]
	thresholdLevel := levels[config.StacktraceLevel]

	return currentLevel >= thresholdLevel
}
