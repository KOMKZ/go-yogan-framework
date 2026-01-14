# Yogan Framework

**不写重复代码，不操心基础设施。** 组件注册即用，配置自动加载，追踪开箱即有。你写业务，框架兜底。

📖 **文档**：[go-yogan-doc-portal.pages.dev](https://go-yogan-doc-portal.pages.dev/)

> ⚠️ **注意**：项目处于快速迭代阶段，API 可能发生变化。

## 安装

```bash
go get github.com/KOMKZ/go-yogan-framework
```

## 脚手架工具 go-ygctl

一条命令，项目就绪：

```bash
# 安装
go install github.com/KOMKZ/go-ygctl@latest

# 创建 HTTP 项目
go-ygctl new http my-api

# 创建 gRPC / CLI / Cron 项目
go-ygctl new grpc my-service
go-ygctl new cli my-tool
go-ygctl new cron my-scheduler
```

生成的项目结构完整可运行：配置文件、路由、健康检查、Docker Compose 一应俱全。

**查看可用组件**：

```bash
go-ygctl component list
```

**获取组件集成指南**：

```bash
go-ygctl component add database
go-ygctl component add redis
go-ygctl component add kafka
```

不用翻文档，命令行直接告诉你怎么接入。

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
