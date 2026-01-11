# Changelog

所有重要变更记录在此文件中。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/)，
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

## [Unreleased]

### Added
- 初始版本，包含核心组件
- application: HTTP/gRPC/CLI/Cron 应用支持
- database: GORM 多数据库连接池
- redis: Redis 客户端管理
- kafka: Kafka 生产者/消费者
- auth: 密码认证 + 登录尝试限制
- jwt: JWT Token 生成/验证/刷新
- middleware: CORS/TraceID/RequestLog/Recovery
- telemetry: OpenTelemetry 集成
- health: 健康检查
- limiter: 限流组件
- breaker: 熔断器
- retry: 重试策略
