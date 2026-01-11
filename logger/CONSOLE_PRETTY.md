# Console Pretty 编码使用示例

## 使用方法

### 1. 在配置文件中使用

```yaml
logger:
  base_log_dir: "logs"
  level: "info"
  encoding: "console_pretty"  # 使用美化控制台格式
  enable_console: true
  enable_level_in_filename: true
  enable_date_in_filename: true
  max_size: 100
  enable_caller: true
  enable_stacktrace: true
  stacktrace_level: "error"
  enable_trace_id: true
```

### 2. 混合使用（文件 JSON，控制台 Pretty）

```yaml
logger:
  base_log_dir: "logs"
  level: "info"
  encoding: "json"                    # 文件使用 JSON（便于解析）
  console_encoding: "console_pretty"  # 控制台使用 Pretty（便于阅读）
  enable_console: true
  enable_caller: true
  enable_stacktrace: true
  stacktrace_level: "error"
  enable_trace_id: true
```

### 3. 代码中使用

```go
package main

import (
	"context"
	"github.com/KOMKZ/go-yogan/logger"
	"go.uber.org/zap"
)

func main() {
	// 初始化 logger（使用 console_pretty）
	logger.InitManager(logger.ManagerConfig{
		BaseLogDir:            "logs",
		Level:                 "info",
		Encoding:              "console_pretty",
		EnableConsole:         true,
		EnableLevelInFilename: true,
		EnableDateInFilename:  true,
		MaxSize:               100,
		EnableCaller:          true,
		EnableStacktrace:      true,
		StacktraceLevel:       "error",
		EnableTraceID:         true,
	})
	defer logger.CloseAll()

	// 普通日志
	logger.Info("order", "订单创建", 
		zap.String("order_id", "001"),
		zap.Float64("amount", 99.99))

	// 带 TraceID 的日志
	ctx := context.WithValue(context.Background(), "trace_id", "trace-abc-123")
	logger.DebugCtx(ctx, "payment", "支付成功",
		zap.String("order_id", "001"),
		zap.Float64("amount", 199.99))

	// 错误日志（带堆栈）
	logger.Error("auth", "登录失败",
		zap.String("user", "admin"),
		zap.String("reason", "密码错误"))
}
```

## 输出效果

### 完整格式（带 TraceID）
```
[🔵INFO]  |  2025-12-20T09:14:58.575+0800  |  trace-abc-123  |  [order]  |  order/manager.go:123  |  订单创建  |  {"order_id":"001","amount":99.99}
```

### 无 TraceID
```
[🟡WARN]  |  2025-12-20T22:06:13.565+0800  |  -  |  [cache]  |  cache/redis.go:89  |  缓存未命中  |  {"key":"user:100"}
```

### 错误日志（带堆栈）
```
[🔴ERRO]  |  2025-12-20T22:12:59.193+0800  |  -  |  [auth]  |  logger/manager.go:293  |  登录失败  |  {"user":"admin","reason":"密码错误"}
github.com/KOMKZ/go-yogan/logger.(*Manager).Error
	/path/to/logger/manager.go:293
github.com/KOMKZ/go-yogan/logger.Error
	/path/to/logger/manager.go:466
...
```

## 各级别 Emoji

| 级别 | Emoji | 显示 |
|------|-------|------|
| Debug | 🟢 | `[🟢DEBU]` |
| Info | 🔵 | `[🔵INFO]` |
| Warn | 🟡 | `[🟡WARN]` |
| Error | 🔴 | `[🔴ERRO]` |
| DPanic | 🟠 | `[🟠DPAN]` |
| Panic | 🟣 | `[🟣PANI]` |
| Fatal | 💀 | `[💀FATA]` |

## 格式说明

```
[Emoji+级别]  |  完整时间戳  |  TraceID  |  [模块名]  |  文件:行号  |  消息  |  JSON字段
     1             2            3           4           5          6         7
```

1. **级别 (Emoji+4字符)**: 带颜色 Emoji 的日志级别
2. **时间戳**: ISO8601 完整时间（含毫秒和时区）
3. **TraceID**: 追踪 ID（无则显示 `-`）
4. **模块名**: 方括号包裹的模块名
5. **位置**: 文件路径:行号
6. **消息**: 日志消息内容
7. **字段**: JSON 格式的额外字段

## 配置选项

| 配置项 | 类型 | 可选值 | 说明 |
|-------|------|--------|------|
| `encoding` | string | `json`, `console`, `console_pretty` | 文件编码格式 |
| `console_encoding` | string | `json`, `console`, `console_pretty` | 控制台编码格式 |

## 最佳实践

1. **生产环境**: 文件使用 `json`（便于日志采集和解析），控制台使用 `console_pretty`（便于实时查看）
2. **开发环境**: 全部使用 `console_pretty`（便于阅读和调试）
3. **日志采集**: 使用 `json` 编码，配合 ELK/Loki 等日志系统
4. **实时监控**: 使用 `console_pretty`，配合 `tail -f` 查看日志文件

