// src/yogan/logger/stacktrace.go
package logger

import (
	"fmt"
	"runtime"
	"strings"
)

// CaptureStacktrace 捕获当前调用栈（支持深度限制）
// skip: 跳过的栈帧数（通常是 2-3，跳过 CaptureStacktrace 和调用者本身）
// depth: 最大深度（0 表示不限制，建议 5-10）
// 返回格式化的堆栈字符串，每行一个调用帧
func CaptureStacktrace(skip int, depth int) string {
	maxDepth := depth
	if maxDepth <= 0 {
		maxDepth = 32 // 默认最大 32 层
	}

	var frames []string
	// 预留更多空间，确保能捕获到足够的帧
	pcs := make([]uintptr, maxDepth*2)
	n := runtime.Callers(skip, pcs)

	if n == 0 {
		return ""
	}

	callersFrames := runtime.CallersFrames(pcs[:n])
	frameCount := 0
	for {
		frame, more := callersFrames.Next()

		// 格式化：函数名
		//         文件:行号
		frames = append(frames, fmt.Sprintf("%s\n\t%s:%d", frame.Function, frame.File, frame.Line))
		frameCount++

		// 达到深度限制或没有更多帧
		if frameCount >= maxDepth || !more {
			break
		}
	}

	return strings.Join(frames, "\n")
}

// shouldCaptureStacktrace 判断当前日志级别是否需要记录堆栈
func shouldCaptureStacktrace(level string, config ManagerConfig) bool {
	if !config.EnableStacktrace {
		return false
	}

	// 定义级别优先级
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
