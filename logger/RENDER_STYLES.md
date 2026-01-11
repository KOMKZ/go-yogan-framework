# Console Pretty 渲染样式配置文档

## 📖 概述

`console_pretty` 编码器支持多种渲染样式，适应不同的终端环境和用户偏好。

---

## 🎨 渲染样式

### 1. 单行样式（single_line）- 默认

**特点**：所有信息在一行显示，适合宽屏幕终端。

**格式**：
```
[🔵INFO]  |  2025-12-23T01:10:01.165+0800  |  trace-id  |  [module]  |  file:line  |  message  |  {"key":"value"}
```

**配置**：
```yaml
logger:
  encoding: console_pretty
  render_style: single_line  # 或不设置（默认）
```

**示例输出**：
```
[🟢DEBU]  |  2025-12-23T01:10:01.165+0800  |         -          |  [gin-route]                |  logger/manager.go:316           |  [GIN-debug] GET /                         --> github.com/KOMKZ/futurelzapi/apps/user-api/internal/handler.(*HomeHandler).Index-fm (4 handlers)
[🔵INFO]  |  2025-12-23T01:10:02.234+0800  |  47dfd756-254f-4f  |  [order]                    |  order/service.go:123            |  订单创建成功  |  {"order_id":"001","amount":99.99}
[🟡WARN]  |  2025-12-23T01:10:03.456+0800  |         -          |  [cache]                    |  cache/redis.go:89               |  缓存未命中  |  {"key":"user:100"}
[🔴ERRO]  |  2025-12-23T01:10:04.789+0800  |  2ebe046c-19e0-47  |  [auth]                     |  auth/service.go:45              |  登录失败  |  {"user":"admin","reason":"密码错误"}
```

---

### 2. 键值对样式（key_value）- 适合小屏幕

**特点**：多行显示，每个字段独立一行，适合小屏幕、手机终端、SSH 窄终端等。

**格式**：
```
🟢 DEBU | 2025-12-23 01:10:01.165
  trace: -
  module: gin-route
  caller: logger/manager.go:316
  message: [GIN-debug] GET / --> handler.Index (4 handlers)
  fields: {"key":"value"}
```

**配置**：
```yaml
logger:
  encoding: console_pretty
  render_style: key_value
```

**示例输出**：
```
🟢 DEBU | 2025-12-23 01:10:01.165
  trace: -
  module: gin-route
  caller: logger/manager.go:316
  message: [GIN-debug] GET / --> handler.Index (4 handlers)

🔵 INFO | 2025-12-23 01:10:02.234
  trace: 47dfd756-254f-4f
  module: order
  caller: order/service.go:123
  message: 订单创建成功
  fields: {"order_id":"001","amount":99.99}

🟡 WARN | 2025-12-23 01:10:03.456
  trace: -
  module: cache
  caller: cache/redis.go:89
  message: 缓存未命中
  fields: {"key":"user:100"}

🔴 ERRO | 2025-12-23 01:10:04.789
  trace: 2ebe046c-19e0-47
  module: auth
  caller: auth/service.go:45
  message: 登录失败
  fields: {"user":"admin","reason":"密码错误"}
  stack:
  goroutine 1 [running]:
  auth.(*Service).Login(...)
    /app/auth/service.go:45
```

---

## 🛠️ 完整配置示例

### 示例 1：开发环境（单行样式）

```yaml
# config/dev.yaml
api_server:
  port: 8080

logger:
  level: debug
  encoding: console_pretty
  render_style: single_line    # 默认单行样式
  enable_console: true
  enable_caller: true
  enable_stacktrace: true
  stacktrace_level: error
```

### 示例 2：小屏幕终端（键值对样式）

```yaml
# config/mobile.yaml
api_server:
  port: 8080

logger:
  level: info
  encoding: console_pretty
  render_style: key_value      # 键值对样式，适合小屏幕
  enable_console: true
  enable_caller: true
  enable_stacktrace: true
  stacktrace_level: error
```

### 示例 3：生产环境（JSON 格式）

```yaml
# config/prod.yaml
api_server:
  port: 8080

logger:
  level: info
  encoding: json               # 生产环境使用 JSON 格式
  enable_console: false        # 不输出到控制台
  enable_caller: true
  enable_stacktrace: true
  stacktrace_level: error
  max_size: 100                # 单文件 100MB
  max_backups: 30              # 保留 30 个备份
  max_age: 7                   # 保留 7 天
  compress: true               # 压缩旧文件
```

---

## 🎯 使用场景

| 场景 | 推荐样式 | 原因 |
|-----|---------|------|
| **宽屏桌面终端** | `single_line` | 所有信息一行显示，扫描速度快 |
| **笔记本电脑** | `single_line` | 屏幕宽度足够 |
| **手机 SSH 终端** | `key_value` | 屏幕窄，单行会换行混乱 |
| **窄窗口终端** | `key_value` | 避免水平滚动 |
| **日志分析** | `single_line` | 便于 grep、awk 处理 |
| **人工阅读** | `key_value` | 层次清晰，易于阅读 |
| **生产环境** | `json` | 机器可读，便于日志收集系统处理 |

---

## 🔧 动态切换

### 方法 1：通过环境变量

```bash
# 单行样式
export LOGGER_RENDER_STYLE=single_line
./app

# 键值对样式
export LOGGER_RENDER_STYLE=key_value
./app
```

### 方法 2：通过配置文件

```bash
# 开发环境
./app --config=config/dev.yaml

# 小屏幕环境
./app --config=config/mobile.yaml
```

---

## 📊 性能对比

| 样式 | 行数 | 字符数 | 性能 | 可读性 |
|-----|------|--------|------|--------|
| `single_line` | 1 | ~200 | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ |
| `key_value` | 5-6 | ~250 | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ |

**说明**：
- `single_line` 性能最优，适合高频日志输出
- `key_value` 可读性最优，适合开发调试和人工阅读

---

## 🎨 Emoji 级别映射

两种样式都使用相同的 Emoji：

| 级别 | Emoji | 颜色 | 说明 |
|-----|-------|------|------|
| DEBUG | 🟢 | 绿色 | 调试信息 |
| INFO | 🔵 | 蓝色 | 一般信息 |
| WARN | 🟡 | 黄色 | 警告信息 |
| ERROR | 🔴 | 红色 | 错误信息 |
| DPANIC | 🟠 | 橙色 | 开发 Panic |
| PANIC | 🟣 | 紫色 | Panic |
| FATAL | 💀 | 骷髅 | 致命错误 |

---

## 📝 注意事项

1. **终端支持**：确保终端支持 UTF-8 和 Emoji 显示
2. **配置优先级**：环境变量 > 配置文件 > 默认值
3. **性能考虑**：`key_value` 样式输出行数更多，高频日志场景建议用 `single_line`
4. **日志收集**：生产环境建议使用 `json` 编码，便于日志收集系统解析
5. **兼容性**：默认为 `single_line`，向后兼容现有配置

---

## 🚀 扩展性

当前架构支持轻松添加新的渲染样式：

1. 在 `encoder.go` 中添加新的 `RenderStyle` 常量
2. 实现对应的 `encode*` 方法
3. 在 `EncodeEntry` 中添加分支
4. 编写测试验证

示例：未来可添加的样式
- `compact`：超紧凑单行（去掉所有空格和分隔符）
- `colorful`：彩色输出（使用 ANSI 颜色代码）
- `table`：表格样式（对齐列）
- `markdown`：Markdown 格式（便于粘贴到文档）

---

**文档版本**：v1.0  
**最后更新**：2025-12-23  
**适用版本**：>=v1.0.0

