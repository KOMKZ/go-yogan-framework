# Kernel Request Log

## 目标

`middleware.RequestLog` 是 HTTP 访问日志中间件：为每个请求输出结构化日志（status、latency、client_ip、method、path、body_size），并按最终状态码分级——5xx=Error、4xx=Warn、其余=Info。应用通过 `middleware.request_log` 配置节声明行为，装配点在 `application/http_server.go`。

## 配置

```yaml
middleware:
  request_log:
    enable: true
    # 整路径跳过：请求执行前判断，无论最终状态码（含 5xx）都不记录。
    # 只用于 health check 等完全不关心的路径。
    skip_paths: ["/health"]
    # 条件跳过：请求执行后按“方法 + 精确路径 + 最终状态码”判断，
    # 仅当三者全部精确命中才抑制这条成功日志。
    skip_rules:
      - method: GET
        path: /api/openapi/v1/render-ops/load
        statuses: [200]
    enable_body: false
    max_body_size: 4096
```

## skip_paths 与 skip_rules 的区别

| 维度 | `skip_paths` | `skip_rules` |
|------|--------------|--------------|
| 判断时机 | 请求执行前 | `c.Next()` 之后（拿最终 status） |
| 匹配方式 | 路径全等 | method（忽略大小写）+ 路径全等 + status 命中集合 |
| 错误可观测性 | 同路径的 401/4xx/5xx 一并被吞 | 错误响应永不抑制 |

## skip_rules 使用约束

- 只做精确匹配：不支持前缀、通配符、正则、采样或动态管理。
- `statuses` 必须显式列出，语义上只允许成功状态码（2xx，如空队列 `204`）；这是“降噪”能力，不是“屏蔽”能力。
- 同一路由的认证失败、限流、4xx、5xx 始终照常分级记录；成功但非规则状态码（如任务真正领取到的 `200`）也始终记录。
- 规则数量预期是个位数；超量增长说明需求本身越界，应先重新评审而不是扩机制。

## 测试

- `middleware/request_log_test.go`：精确命中、同路径不同 method/status、401/500 不被吞、大小写不敏感、`skip_paths` 语义不回归。
- `application/middleware_config_test.go`：YAML → `AppConfig` 解析不丢字段。
- `application/http_server_test.go`：`NewHTTPServer` 装配后端到端生效。
