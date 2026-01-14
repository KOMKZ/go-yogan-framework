# Yogan Framework

**不写重复代码，不操心基础设施。** 组件注册即用，配置自动加载，追踪开箱即有。你写业务，框架兜底。

📖 **文档**：[go-yogan-doc-portal.pages.dev](https://go-yogan-doc-portal.pages.dev/)

> ⚠️ **注意**：项目处于快速迭代阶段，API 可能发生变化。

## 安装

```bash
go get github.com/KOMKZ/go-yogan-framework
```

## 核心组件

| 组件 | 说明 |
|------|------|
| application | 应用生命周期管理（HTTP/gRPC/CLI/Cron） |
| component | 组件接口定义 |
| config | 配置加载（YAML + 环境变量） |
| logger | 结构化日志（Zap） |
| database | GORM 数据库连接池 |
| redis | Redis 客户端管理 |
| grpc | gRPC 服务端/客户端 |
| kafka | Kafka 生产者/消费者 |
| auth | 认证服务（密码/OAuth） |
| jwt | JWT Token 管理 |
| middleware | HTTP 中间件（CORS/TraceID/日志） |
| telemetry | OpenTelemetry 分布式追踪 |
| health | 健康检查 |
| limiter | 限流（令牌桶/滑动窗口） |
| breaker | 熔断器 |
| retry | 重试策略 |

## 快速开始

```go
package main

import (
    "github.com/KOMKZ/go-yogan-framework/application"
    "github.com/KOMKZ/go-yogan-framework/database"
    "github.com/KOMKZ/go-yogan-framework/redis"
)

func main() {
    app := application.New("./configs", "MY_APP", nil)
    
    app.Register(
        database.NewComponent(),
        redis.NewComponent(),
    )
    
    app.Run()
}
```

## 协议

[MIT License](LICENSE)
