# HealthChecker 接口

```go
// 组件可选实现 component.HealthChecker 接口

func (m *Manager) Name() string {
    return "mypackage"
}

func (m *Manager) Check(ctx context.Context) error {
    return m.conn.Ping(ctx)
}
```

## health 配置段

```yaml
health:
  enabled: true          # 聚合器开关（默认 true）
  timeout: 5s            # 单次健康聚合探测超时（默认 5s）
  listen: "0.0.0.0:9203" # 非 HTTP 应用的独立 listener 地址；空 = 不启动（HTTP 应用在主端口挂 /health 路由）
```

配置错误（enabled 时 timeout 非正、listen 非法 host:port）在启动期 `Validate` 拒绝，不做静默回退。

## Worker/Cron 独立健康 listener

HTTP 应用由 `middleware.RegisterHealthRoutes` 在主端口挂 `/health`；worker/cron 类非 HTTP 应用
由框架在队列服务启动后按 `health.listen` 开启独立 listener（`application/worker_health_server.go`）：

| 路由 | 语义 | 状态码 |
|---|---|---|
| `/health` | 全量探测（health.Response JSON，与 HTTP 应用同构） | 200 healthy/degraded；503 unhealthy |
| `/health/liveness` | 进程与事件循环存活，不探测依赖 | 恒 200 |
| `/health/readiness` | 依赖就绪（queue broker / database / redis） | 200 healthy；503 其余 |

行为口径：

- **worker 的健康语义是「还能干活」而非「进程活着」**；readiness 含组件依赖探测，
  进程级探活（pgrep）作为唯一健康信号是被否决的反模式。
- listener 在 `RunNonBlocking` 内先于 `OnReady` 启动、`gracefulShutdown` 先于组件关停，
  滚动发布期间旧实例不会被判活。
- 组件探活项按配置存在性注册（`queue.HealthChecker` / `database.HealthChecker` / `redis.HealthChecker`）；
  未配置的组件跳过，健康检查不因未使用组件而失败。
- 部署链路（hrise-ops backend deploy 激活脚本健康轮询、compose healthcheck）以 `/health` 2xx 为门禁。

上游：`WorkerApplication`；下游：compose healthcheck、激活脚本健康轮询、运维探活。
