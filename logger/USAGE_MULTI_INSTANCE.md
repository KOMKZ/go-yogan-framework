# Logger Manager 多实例使用示例

## 场景：应用日志 + 审计日志分离

```go
package main

import (
    "context"
    "github.com/KOMKZ/go-yogan/logger"
    "go.uber.org/zap"
)

func main() {
    // ============================================
    // 方式1：全局 Manager（现有方式，保持兼容）
    // ============================================
    
    logger.InitManager(logger.ManagerConfig{
        BaseLogDir: "logs/app",
        Level:      "info",
        Encoding:   "json",
    })
    
    // 使用包级函数
    logger.Info("order", "订单创建", zap.String("id", "001"))
    logger.Error("auth", "登录失败", zap.String("user", "admin"))
    
    defer logger.CloseAll()
    
    // ============================================
    // 方式2：独立 Manager（新增能力）
    // ============================================
    
    // 应用日志 Manager
    appManager := logger.NewManager(logger.ManagerConfig{
        BaseLogDir:            "logs/app",
        Level:                 "info",
        Encoding:              "json",
        EnableConsole:         true,
        EnableLevelInFilename: true,
        EnableDateInFilename:  true,
        DateFormat:            "2006-01-02",
        MaxSize:               100,
    })
    
    // 审计日志 Manager（单独配置）
    auditManager := logger.NewManager(logger.ManagerConfig{
        BaseLogDir:            "logs/audit",
        Level:                 "info",
        Encoding:              "json",
        EnableConsole:         false, // 审计日志不输出到控制台
        EnableLevelInFilename: true,
        EnableDateInFilename:  true,
        DateFormat:            "2006-01-02",
        MaxSize:               500,   // 审计日志单文件更大
        Compress:              true,  // 审计日志启用压缩
    })
    
    // 独立使用
    appManager.Info("order", "订单创建", zap.String("order_id", "O12345"))
    auditManager.Info("security", "用户登录", 
        zap.String("user", "admin"),
        zap.String("ip", "192.168.1.100"))
    
    // 带 TraceID
    ctx := context.WithValue(context.Background(), "trace_id", "abc-123-xyz")
    appManager.DebugCtx(ctx, "payment", "支付成功", zap.Float64("amount", 99.99))
    
    // 关闭
    defer appManager.CloseAll()
    defer auditManager.CloseAll()
    
    // ============================================
    // 方式3：全局 + 独立共存
    // ============================================
    
    // 全局用于通用日志
    logger.Info("system", "系统启动")
    
    // 审计日志独立管理
    auditManager.Info("audit", "关键操作", zap.String("action", "delete_user"))
}
```

## 场景：测试中使用独立 Manager

```go
func TestMyService(t *testing.T) {
    // 每个测试用独立的 Manager，互不干扰
    testManager := logger.NewManager(logger.ManagerConfig{
        BaseLogDir: t.TempDir(), // 临时目录
        Level:      "debug",
        Encoding:   "json",
    })
    defer testManager.CloseAll()
    
    // 使用独立 Manager
    service := NewService(testManager)
    service.DoSomething()
    
    // 验证日志文件
    // ...
}
```

## 场景：动态配置热更新

```go
// 创建独立 Manager
manager := logger.NewManager(logger.DefaultManagerConfig())

// 运行时热更新配置
newConfig := logger.ManagerConfig{
    BaseLogDir: "logs/updated",
    Level:      "debug", // 动态调整为 debug
    Encoding:   "console",
}

err := manager.ReloadConfig(newConfig)
if err != nil {
    log.Fatal(err)
}

// 重载后生效
manager.Debug("test", "现在可以看到 debug 日志了")
```

## API 对比

| 功能 | 全局方式 | 实例方式 |
|------|---------|---------|
| 初始化 | `InitManager(cfg)` | `NewManager(cfg)` |
| 获取Logger | `GetLogger("module")` | `manager.GetLogger("module")` |
| Info日志 | `Info("module", "msg")` | `manager.Info("module", "msg")` |
| TraceID | `DebugCtx(ctx, "module", "msg")` | `manager.DebugCtx(ctx, "module", "msg")` |
| 关闭 | `CloseAll()` | `manager.CloseAll()` |
| 热更新 | `ReloadConfig(cfg)` | `manager.ReloadConfig(cfg)` |

## 目录结构示例

```
logs/
├── app/                  # 应用日志
│   ├── order/
│   │   ├── order-info-2025-12-20.log
│   │   └── order-error-2025-12-20.log
│   └── payment/
│       └── payment-info-2025-12-20.log
└── audit/                # 审计日志
    └── security/
        └── security-info-2025-12-20.log
```

## 优势

✅ **灵活性**：支持多实例，满足复杂场景  
✅ **兼容性**：现有代码无需修改  
✅ **可测试**：测试用独立实例，互不干扰  
✅ **解耦**：Manager 不依赖全局变量

