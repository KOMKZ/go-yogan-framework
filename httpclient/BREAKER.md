# HTTPClient 熔断器集成

HTTPClient 支持可插拔的熔断器（Circuit Breaker）集成，提供自动故障隔离和降级能力。

## 设计理念

- ✅ **可插拔** - 通过接口解耦，不强依赖 breaker 包
- ✅ **零侵入** - 未配置时自动透传，不影响性能
- ✅ **灵活配置** - 支持全局和请求级配置
- ✅ **降级友好** - 支持自定义 fallback 逻辑
- ✅ **与 Retry 协同** - 熔断器 + 重试智能组合

## 快速开始

### 基础用法

```go
import (
    "github.com/KOMKZ/go-yogan-framework/httpclient"
    "github.com/KOMKZ/go-yogan-framework/breaker"
)

// 1. 创建熔断器管理器
breakerConfig := breaker.DefaultConfig()
breakerConfig.Enabled = true
breakerMgr, _ := breaker.NewManager(breakerConfig)

// 2. 创建 HTTP Client（全局启用熔断器）
client := httpclient.NewClient(
    httpclient.WithBaseURL("https://api.example.com"),
    httpclient.WithBreaker(breakerMgr),
)

// 3. 正常使用（自动熔断保护）
resp, err := client.Get(ctx, "/users/123")
```

### 自定义资源名称

```go
// 默认使用 URL 作为资源名称
// 可以自定义资源名称（用于分组监控）
resp, err := client.Get(ctx, "/orders/123",
    httpclient.WithBreakerResource("order-service"),
)
```

### 降级逻辑

```go
// 设置降级逻辑（熔断时返回缓存数据）
fallback := func(ctx context.Context, err error) (*httpclient.Response, error) {
    // 从缓存获取数据
    cachedData := cache.Get("user:123")
    return &httpclient.Response{
        StatusCode: 200,
        Body:       cachedData,
    }, nil
}

client := httpclient.NewClient(
    httpclient.WithBreaker(breakerMgr),
    httpclient.WithBreakerFallback(fallback),
)
```

### 请求级禁用熔断器

```go
// 某些关键请求不希望被熔断
resp, err := client.Post(ctx, "/critical-operation",
    httpclient.WithJSON(data),
    httpclient.DisableBreaker(), // 禁用熔断器
)
```

## API 文档

### Option 配置

```go
// WithBreaker 设置熔断器管理器
func WithBreaker(manager BreakerManager) Option

// WithBreakerResource 设置资源名称（默认使用 URL）
func WithBreakerResource(resource string) Option

// WithBreakerFallback 设置降级逻辑
func WithBreakerFallback(fallback func(ctx context.Context, err error) (*Response, error)) Option

// DisableBreaker 禁用熔断器（请求级）
func DisableBreaker() Option
```

### BreakerManager 接口

```go
type BreakerManager interface {
    // Execute 执行受保护的操作
    Execute(ctx context.Context, req *breaker.Request) (interface{}, error)
    
    // IsEnabled 检查熔断器是否启用
    IsEnabled() bool
    
    // GetState 获取资源的当前状态
    GetState(resource string) breaker.State
}
```

## 使用场景

### 场景一：保护外部服务

```go
// 保护不稳定的外部 API
breakerConfig := breaker.Config{
    Enabled: true,
    Default: breaker.ResourceConfig{
        Strategy:           "error_rate",
        ErrorRateThreshold: 0.5,  // 错误率 50%
        Timeout:            30 * time.Second,
        HalfOpenRequests:   3,
    },
}

breakerMgr, _ := breaker.NewManager(breakerConfig)

client := httpclient.NewClient(
    httpclient.WithBaseURL("https://unstable-api.example.com"),
    httpclient.WithBreaker(breakerMgr),
    httpclient.WithRetry(retry.HTTPDefaults...),
)

// 自动熔断 + 重试
resp, err := client.Get(ctx, "/data")
```

### 场景二：多服务独立熔断

```go
// 为不同的服务配置不同的熔断策略
breakerConfig := breaker.Config{
    Enabled: true,
    Resources: map[string]breaker.ResourceConfig{
        "user-service": {
            Strategy:           "error_rate",
            ErrorRateThreshold: 0.3,
            Timeout:            10 * time.Second,
        },
        "payment-service": {
            Strategy:           "error_rate",
            ErrorRateThreshold: 0.1, // 支付服务更严格
            Timeout:            60 * time.Second,
        },
    },
}

breakerMgr, _ := breaker.NewManager(breakerConfig)

userClient := httpclient.NewClient(
    httpclient.WithBaseURL("https://user-api.example.com"),
    httpclient.WithBreaker(breakerMgr),
    httpclient.WithBreakerResource("user-service"),
)

paymentClient := httpclient.NewClient(
    httpclient.WithBaseURL("https://payment-api.example.com"),
    httpclient.WithBreaker(breakerMgr),
    httpclient.WithBreakerResource("payment-service"),
)
```

### 场景三：熔断 + Fallback

```go
// 熔断时返回默认值
fallback := func(ctx context.Context, err error) (*httpclient.Response, error) {
    log.Warn("Circuit breaker triggered, using fallback", "error", err)
    
    // 返回默认响应
    return &httpclient.Response{
        StatusCode: 200,
        Body:       []byte(`{"status": "degraded", "data": []}`),
    }, nil
}

client := httpclient.NewClient(
    httpclient.WithBreaker(breakerMgr),
    httpclient.WithBreakerFallback(fallback),
)

// 即使熔断也能正常返回
resp, err := client.Get(ctx, "/api/list")
// err == nil，返回降级数据
```

### 场景四：熔断器 + 重试协同

```go
// 熔断器在外层，重试在内层
// 执行顺序：Retry → Breaker → HTTP Request

client := httpclient.NewClient(
    httpclient.WithBreaker(breakerMgr), // 全局熔断器
    httpclient.WithRetry(               // 全局重试
        retry.MaxAttempts(3),
        retry.Backoff(retry.ExponentialBackoff(time.Second)),
    ),
)

// 执行流程：
// 1. Retry 尝试第 1 次 → 进入 Breaker → HTTP 请求失败
// 2. Retry 尝试第 2 次 → 进入 Breaker → HTTP 请求失败
// 3. Retry 尝试第 3 次 → 进入 Breaker → HTTP 请求成功
// 4. 如果所有重试都失败，Breaker 会记录失败并可能触发熔断

resp, err := client.Get(ctx, "/api/data")
```

## 工作原理

### 请求流程

```
┌──────────────────────────────────────────────────────┐
│                  HTTP Client.Do()                    │
└──────────────────────────────────────────────────────┘
                          │
                          ▼
┌──────────────────────────────────────────────────────┐
│              检查是否启用熔断器？                      │
│  - breakerManager != nil                             │
│  - !breakerDisabled                                  │
│  - breakerManager.IsEnabled()                        │
└──────────────────────────────────────────────────────┘
          │                                  │
        Yes                               No
          │                                  │
          ▼                                  ▼
┌─────────────────────┐          ┌──────────────────┐
│ executeWithBreaker()│          │   doRequest()    │
└─────────────────────┘          └──────────────────┘
          │                                  │
          ▼                                  │
┌─────────────────────┐                     │
│  Breaker.Execute()  │                     │
│  - 检查熔断状态     │                     │
│  - 记录指标         │                     │
│  - 触发熔断？       │                     │
└─────────────────────┘                     │
          │                                  │
          ▼                                  │
┌─────────────────────┐                     │
│   doRequest()       │◄────────────────────┘
│  执行实际 HTTP 请求 │
└─────────────────────┘
          │
          ▼
┌─────────────────────┐
│  检查响应状态码      │
│  5xx → 返回 error    │
│  2xx/3xx/4xx → OK    │
└─────────────────────┘
```

### 错误处理

- **5xx 错误** → 被视为失败，传递给熔断器统计
- **4xx 错误** → 被视为客户端错误，不触发熔断
- **网络错误** → 被视为失败，传递给熔断器统计
- **超时错误** → 被视为失败，传递给熔断器统计

### 与 Retry 协同

**执行顺序**: `Retry → Breaker → HTTP Request`

- Retry 在外层，控制整体重试逻辑
- Breaker 在内层，每次重试都会进入熔断器
- 如果熔断器打开，Retry 会立即收到错误，停止重试

## 最佳实践

### 1. 选择合适的熔断策略

```go
// 高流量服务：使用错误率策略
breaker.ResourceConfig{
    Strategy:           "error_rate",
    ErrorRateThreshold: 0.5,
    MinRequests:        20, // 避免小流量误判
}

// 低流量服务：使用连续失败策略
breaker.ResourceConfig{
    Strategy:            "consecutive",
    ConsecutiveFailures: 5,
}

// 超时敏感：使用慢调用策略
breaker.ResourceConfig{
    Strategy:          "slow_call_rate",
    SlowCallThreshold: 2 * time.Second,
    SlowRateThreshold: 0.5,
}
```

### 2. 合理设置资源名称

```go
// ❌ 不推荐：每个 URL 都是独立资源
client.Get(ctx, "/users/123")  // 资源: /users/123
client.Get(ctx, "/users/456")  // 资源: /users/456

// ✅ 推荐：按服务分组
client.Get(ctx, "/users/123",
    httpclient.WithBreakerResource("user-service"),
)
client.Get(ctx, "/users/456",
    httpclient.WithBreakerResource("user-service"),
)
```

### 3. 降级逻辑设计

```go
// ✅ 推荐：提供合理的默认值
fallback := func(ctx context.Context, err error) (*httpclient.Response, error) {
    // 返回空列表而不是错误
    return &httpclient.Response{
        StatusCode: 200,
        Body:       []byte(`[]`),
    }, nil
}

// ❌ 不推荐：降级逻辑再次失败
fallback := func(ctx context.Context, err error) (*httpclient.Response, error) {
    // 调用另一个可能也会失败的服务
    return backupClient.Get(ctx, "/backup")
}
```

### 4. 监控熔断状态

```go
// 订阅熔断器事件
eventBus := breakerMgr.GetEventBus()
eventBus.Subscribe(breaker.EventListenerFunc(func(event breaker.Event) {
    if event.Type() == breaker.EventStateChanged {
        stateEvent := event.(*breaker.StateChangedEvent)
        log.Warn("Circuit breaker state changed",
            "resource", stateEvent.Resource(),
            "from", stateEvent.FromState,
            "to", stateEvent.ToState,
            "reason", stateEvent.Reason,
        )
        // 发送告警
        alerting.Send("Circuit breaker triggered: " + stateEvent.Resource())
    }
}))
```

## 测试覆盖率

熔断器集成的测试覆盖包括：
- ✅ 基础配置测试
- ✅ 熔断触发测试
- ✅ 降级逻辑测试
- ✅ 与 Retry 协同测试
- ✅ 资源名称配置测试
- ✅ 禁用熔断器测试

```bash
$ go test -cover -run TestClient_Do_WithBreaker
ok      github.com/KOMKZ/go-yogan-framework/httpclient
```

## 相关文档

- [Breaker 组件文档](../breaker/README.md)
- [Retry 工具库](../retry/README.md)
- [HTTPClient 主文档](./README.md)

