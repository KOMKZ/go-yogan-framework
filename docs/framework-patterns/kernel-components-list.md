# 内核组件列表

> 开发应用时优先使用内核组件，不要造轮子

## 核心组件

| 组件 | 包路径 | 用途 |
|------|--------|------|
| application | `application/` | 应用生命周期管理 |
| di | `di/` | 依赖注入（samber/do 封装） |
| config | `config/` | 多源配置加载 |
| logger | `logger/` | 结构化日志（zap） |
| database | `database/` | MySQL/PostgreSQL 多实例 |
| redis | `redis/` | Redis 多实例连接 |
| cache | `cache/` | 缓存编排层（多后端、事件失效） |
| jwt | `jwt/` | Token 生成/验证 |
| auth | `auth/` | 认证服务（登录/密码） |
| event | `event/` | 事件分发器 |
| queue | `queue/` | 队列任务抽象、Asynq adapter、worker handler registry |

## 基础设施组件

| 组件 | 包路径 | 用途 |
|------|--------|------|
| grpc | `grpc/` | gRPC 服务端/客户端 |
| kafka | `kafka/` | Kafka 生产者/消费者 |
| limiter | `limiter/` | 限流（令牌桶/滑动窗口/自适应） |
| breaker | `breaker/` | 熔断器 |
| health | `health/` | 健康检查 |
| telemetry | `telemetry/` | OpenTelemetry 追踪/指标 |
| httpclient | `httpclient/` | HTTP 客户端（带熔断/重试） |
| retry | `retry/` | 重试策略 |
| middleware | `middleware/` | HTTP 中间件 |

## 独立组件

| 组件 | 仓库 | 用途 |
|------|------|------|
| email | `go-yogan-component-email` | 邮件发送（SMTP/Mandrill） |
