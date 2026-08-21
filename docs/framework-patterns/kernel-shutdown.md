# Shutdownable 接口

## 基本范式

```go
type Manager struct {
    conn *Connection
    log  *logger.CtxZapLogger
}

// Shutdown 实现 do.Shutdownable 接口
func (m *Manager) Shutdown() error {
    if m.conn != nil {
        return m.conn.Close()
    }
    return nil
}
```

## nil 实例防护（必须）

samber/do v2.0.0 在容器关闭时会对「已解析的惰性服务」调用 `Shutdown()`，即使该服务解析结果是 nil 实例（Provider 返回 `(nil, nil)` 表示"未配置"）。因此：

- **所有实现 do.Shutdownable 的具体类型（Manager/Server 等），`Shutdown()`/`Close()` 开头必须加 nil 接收者防护**：

```go
func (m *Manager) Close() error {
    // Nil-safe: the provider may resolve to nil when not configured,
    // and samber/do still calls Shutdown on the nil instance.
    if m == nil {
        return nil
    }
    // ...
}
```

- 接口类型（如 `jwt.TokenManager`、`event.Dispatcher`）解析为 nil 接口时不会命中 do 的类型分支，天然安全。
- 已防护：`limiter`、`database`、`redis`、`kafka`、`swagger`、`telemetry.Manager`、`telemetry.MetricsManager`、`governance` 的 Shutdown/Close。
- 回归测试：`application/http_app_test.go` 的 `TestApplication_GracefulShutdown_NoPanicWithUnconfiguredComponents`（未配置任何可选组件时优雅停机不 panic）。
- 新增可空解析的 Shutdownable 具体类型时，必须同步加 nil 防护并在该测试中触发解析。

## 应用级 Shutdown 语义

| 应用类型 | `Shutdown()` 语义 | 说明 |
|----------|-------------------|------|
| HTTP `Application` | `Cancel()` + `gracefulShutdown()` | 停 HTTP server + 关 DI 容器，返回 error |
| Cron `CronApplication` | `Cancel()` + `gracefulShutdown()` | 先停 scheduler，再走 Base 关闭 |
| gRPC `GRPCApplication` | `Cancel()` + `gracefulShutdown()` | 先 `grpc.Server.Stop()`，再走 Base 关闭 |
| CLI `CLIApplication` | 无 Shutdown()，`Execute()` 结束即关闭 | 同步一次性执行语义 |

规则：

- **`Shutdown()` 必须做完整清理**（关 server/worker + 关 DI 容器），不能只 Cancel context。`RunNonBlocking()` + `Shutdown()` 是测试与程序控制的合法用法，不允许泄漏资源。
- **幂等**：`BaseApplication.Shutdown(timeout)` 用 `sync.Once` 保证只执行一次（手动 `Shutdown()` 与阻塞 `Run()` 退出路径并发时安全）；Cron 的 `gracefulShutdown` 同样用 `sync.Once` 包裹，因为 gocron 的 `Shutdown()` 不可重入（`stopErrCh` 只有一个接收者，并发调用会卡到超时）。
- **服务型应用统一入口**：`ServiceApp interface { Run() error }`，HTTP/Cron/gRPC 均实现（编译期断言在 `base_app.go`）；CLI 保持 `Execute() error`。
- 新增应用类型时：服务型实现 `Run() error`；持自有资源（scheduler/server）的应用，`gracefulShutdown` 需自行先关资源再调 `BaseApplication.Shutdown`，并评估幂等防护。
