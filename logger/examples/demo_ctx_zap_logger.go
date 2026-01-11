// src/pkg/logger/examples/demo_ctx_zap_logger.go
package main

import (
	"context"
	"fmt"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"go.uber.org/zap"
)

func main() {
	// ========================================
	// 1. 初始化全局 Logger Manager
	// ========================================
	cfg := logger.DefaultManagerConfig()
	cfg.Level = "debug"
	cfg.EnableConsole = true
	cfg.ConsoleEncoding = "console_pretty"
	cfg.EnableTraceID = true
	logger.InitManager(cfg)
	defer logger.CloseAll()

	fmt.Println("========================================")
	fmt.Println("Demo: CtxZapLogger 使用示例")
	fmt.Println("========================================\n")

	// ========================================
	// 2. 创建 CtxZapLogger（绑定 module）
	// ========================================
	userLogger := logger.GetLogger("user")
	orderLogger := logger.GetLogger("order")

	// ========================================
	// 3. 使用 Logger（自动提取 TraceID）
	// ========================================

	// 场景1：没有 TraceID 的 Context
	ctx1 := context.Background()
	userLogger.DebugCtx(ctx1, "User login",
		zap.String("username", "zhangsan"),
		zap.String("ip", "192.168.1.100"))

	fmt.Println()

	// 场景2：带 TraceID 的 Context
	ctx2 := context.WithValue(context.Background(), "trace_id", "req-12345")
	userLogger.DebugCtx(ctx2, "创建订单",
		zap.Int64("user_id", 1001),
		zap.String("product", "iPhone 15"))

	orderLogger.DebugCtx(ctx2, "订单创建成功",
		zap.Int64("order_id", 5001),
		zap.Float64("amount", 7999.00))

	fmt.Println()

	// 场景3：链式调用（With）
	ctx3 := context.WithValue(context.Background(), "trace_id", "req-67890")
	orderProcessLogger := orderLogger.With(
		zap.Int64("order_id", 5002),
		zap.String("status", "processing"))

	orderProcessLogger.DebugCtx(ctx3, "开始处理订单")
	orderProcessLogger.DebugCtx(ctx3, "库存检查完成")
	orderProcessLogger.DebugCtx(ctx3, "订单处理完成")

	fmt.Println()

	// 场景4：不同日志级别
	ctx4 := context.WithValue(context.Background(), "trace_id", "req-99999")
	userLogger.DebugCtx(ctx4, "调试信息", zap.String("debug_key", "debug_value"))
	userLogger.DebugCtx(ctx4, "一般信息", zap.String("info_key", "info_value"))
	userLogger.WarnCtx(ctx4, "警告信息", zap.String("warn_key", "warn_value"))
	userLogger.ErrorCtx(ctx4, "错误信息", zap.String("error_key", "error_value"))

	fmt.Println("\n========================================")
	fmt.Println("✅ Demo 完成")
	fmt.Println("========================================")
}
