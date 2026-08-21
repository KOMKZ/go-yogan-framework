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
