# 注册 Provider

## core_registrar.go

```go
// di/core_registrar.go

func RegisterCoreProviders(injector *do.RootScope, opts ConfigOptions) {
    // ... 其他组件 ...
    do.Provide(injector, ProvideMyManager)
}
```

## 组件依赖层级

```
Layer 0: config（无依赖）
Layer 1: logger（依赖 config）
Layer 2: database, redis, jwt, auth, grpc, kafka, event
Layer 3: limiter, telemetry, health, cache
```
