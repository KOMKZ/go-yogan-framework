package main

import (
	"context"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"go.uber.org/zap"
)

func main() {
	// 初始化 logger（使用 console_pretty）
	logger.InitManager(logger.ManagerConfig{
		BaseLogDir:      "logs",
		Level:           "info",
		Encoding:        "json",           // 文件用 JSON
		ConsoleEncoding: "console_pretty", // 控制台用 Pretty
		EnableConsole:   true,
		MaxSize:         10,
		EnableCaller:    true,
		EnableTraceID:   true,
	})
	defer logger.CloseAll()

	// 演示1: 直接使用 MustGetLogger（已自动包含 module 字段）
	orderLogger := logger.GetLogger("order")
	orderLogger.DebugCtx(context.Background(), "Order creation", zap.String("order_id", "001"), zap.Float64("amount", 99.99))

	// 演示2: 使用包级别函数
	logger.Info("payment", "Payment processing", zap.String("method", "alipay"))

	// 演示3: 带 TraceID
	ctx := context.WithValue(context.Background(), "trace_id", "trace-abc-123")
	logger.DebugCtx(ctx, "auth", "User login", zap.String("user", "admin"))

	// 演示4: WithFields 在已有 module 基础上添加更多字段
	cacheLogger := logger.WithFields("cache", zap.String("region", "cn-east"))
	cacheLogger.DebugCtx(context.Background(), "缓存命中", zap.String("key", "user:100"))
}
