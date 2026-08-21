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
