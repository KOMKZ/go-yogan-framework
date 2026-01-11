package logger

import (
	"strings"
)

// GinLogWriter Gin 日志适配器（实现 io.Writer 接口）
// 将 Gin 的文本日志适配到自定义 Logger 组件
type GinLogWriter struct {
	module string // 日志模块名（如 gin-route、gin-internal）
}

// NewGinLogWriter 创建 Gin 日志适配器
// module: 日志模块名，用于区分不同来源的 Gin 日志
//   - "gin-route": 路由注册日志
//   - "gin-internal": 其他内核日志
func NewGinLogWriter(module string) *GinLogWriter {
	return &GinLogWriter{module: module}
}

// Write 实现 io.Writer 接口
// Gin 框架会调用这个方法写入日志
// 将 Gin 的文本日志转换为结构化日志输出
func (w *GinLogWriter) Write(p []byte) (n int, err error) {
	// 去除首尾空格和换行符
	msg := strings.TrimSpace(string(p))
	if msg == "" {
		return len(p), nil
	}

	// 根据日志内容分类处理
	if strings.Contains(msg, "[GIN-debug]") {
		// 路由注册日志：使用 Debug 级别
		Debug(w.module, msg)
	} else if strings.Contains(msg, "[GIN]") {
		// HTTP 请求日志（如果使用 gin.Logger 中间件）：使用 Info 级别
		Info(w.module, msg)
	} else if strings.Contains(msg, "[Recovery]") || strings.Contains(msg, "panic recovered") {
		// Recovery 日志：使用 Error 级别
		Error(w.module, msg)
	} else {
		// 其他日志：使用 Info 级别
		Info(w.module, msg)
	}

	return len(p), nil
}

