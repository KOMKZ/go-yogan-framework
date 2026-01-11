# HTTPClient 使用指南

Go-Yogan 框架的统一 HTTP 客户端封装，基于函数式选项模式设计，支持 Retry（重试）和 Breaker（熔断器）。

## 📖 目录

- [快速开始](#快速开始)
- [基础使用](#基础使用)
- [高级特性](#高级特性)
- [Retry 集成](#retry-集成)
- [Breaker 集成](#breaker-集成)
- [完整示例](#完整示例)
- [最佳实践](#最佳实践)

---

## 快速开始

### 安装

```bash
go get github.com/KOMKZ/go-yogan-framework/httpclient
```

### 最简单的使用

```go
package main

import (
    "context"
    "fmt"
    
    "github.com/KOMKZ/go-yogan-framework/httpclient"
)

func main() {
    // 创建 Client
    client := httpclient.NewClient()
    
    // 发送 GET 请求
    resp, err := client.Get(context.Background(), "https://api.example.com/users/123")
    if err != nil {
        panic(err)
    }
    defer resp.Close()
    
    fmt.Println("Response:", resp.String())
}
```

---

## 基础使用

### 1. 创建 Client

```go
// 方式 1: 无配置创建（使用默认值）
client := httpclient.NewClient()

// 方式 2: 带配置创建
client := httpclient.NewClient(
    httpclient.WithBaseURL("https://api.example.com"),
    httpclient.WithTimeout(10 * time.Second),
    httpclient.WithHeader("User-Agent", "MyApp/1.0"),
)
```

**推荐做法**：创建全局 Client 实例，复用连接池

```go
// 在包级别定义
var apiClient = httpclient.NewClient(
    httpclient.WithBaseURL("https://api.example.com"),
    httpclient.WithTimeout(10 * time.Second),
)

// 在函数中使用
func GetUser(ctx context.Context, id int) (*User, error) {
    return httpclient.Get[User](apiClient, ctx, fmt.Sprintf("/users/%d", id))
}
```

### 2. 发送 GET 请求

```go
// 简单 GET
resp, err := client.Get(ctx, "/users/123")

// 带查询参数
resp, err := client.Get(ctx, "/users",
    httpclient.WithQuery("page", "1"),
    httpclient.WithQuery("limit", "20"),
)

// 带 Headers
resp, err := client.Get(ctx, "/users/123",
    httpclient.WithHeader("Authorization", "Bearer token"),
)
```

### 3. 发送 POST 请求

```go
// 发送 JSON
data := map[string]interface{}{
    "name":  "Alice",
    "email": "alice@example.com",
}

req := httpclient.NewPostRequest("/users")
req.WithJSON(data)

resp, err := client.Do(ctx, req)
```

### 4. 发送 PUT/DELETE 请求

```go
// PUT 请求
req := httpclient.NewPutRequest("/users/123")
req.WithJSON(updatedData)
resp, err := client.Do(ctx, req)

// DELETE 请求
resp, err := client.Delete(ctx, "/users/123")
```

### 5. 使用泛型自动反序列化

```go
type User struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

// 自动反序列化到 User 类型
user, err := httpclient.Get[User](client, ctx, "/users/123")
if err != nil {
    return err
}

fmt.Printf("User: %s <%s>\n", user.Name, user.Email)
```

### 6. 处理响应

```go
resp, err := client.Get(ctx, "/users/123")
if err != nil {
    return err
}
defer resp.Close()

// 检查状态码
if !resp.IsSuccess() {
    return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
}

// 获取字符串
body := resp.String()

// 获取字节数组
bytes := resp.Bytes()

// 反序列化 JSON
var user User
if err := resp.JSON(&user); err != nil {
    return err
}

// 获取 Header
contentType := resp.Headers.Get("Content-Type")
```

---

## 高级特性

### 1. Request 显式构建与复用

```go
// 构建可复用的 Request
req := httpclient.NewPostRequest("/users")
req.WithHeader("Authorization", "Bearer token")
req.WithHeader("X-Request-ID", uuid.New().String())
req.WithJSON(userData)

// 多次使用
resp1, err := client.Do(ctx, req)
// Request 可以复用
resp2, err := client.Do(ctx, req)
```

### 2. Options 复用与组合

```go
// 定义可复用的 Options
var (
    // 认证 Option
    WithAuth = httpclient.WithHeader("Authorization", "Bearer "+getToken())
    
    // 通用 Headers
    WithCommonHeaders = []httpclient.Option{
        httpclient.WithHeader("User-Agent", "MyApp/1.0"),
        httpclient.WithHeader("Accept", "application/json"),
    }
)

// 组合使用
resp, err := client.Get(ctx, "/api/users",
    WithAuth,
    WithCommonHeaders[0],
    WithCommonHeaders[1],
    httpclient.WithQuery("page", "1"),
)

// 或封装为函数
func withStandardOptions(opts ...httpclient.Option) []httpclient.Option {
    base := []httpclient.Option{WithAuth, WithCommonHeaders[0], WithCommonHeaders[1]}
    return append(base, opts...)
}

resp, err := client.Get(ctx, "/api/users", 
    withStandardOptions(
        httpclient.WithQuery("page", "1"),
    )...,
)
```

### 3. 请求前/后钩子

```go
client := httpclient.NewClient(
    // 请求前钩子（添加签名）
    httpclient.WithBeforeRequest(func(req *http.Request) error {
        signature := generateSignature(req)
        req.Header.Set("X-Signature", signature)
        return nil
    }),
    
    // 响应后钩子（记录日志）
    httpclient.WithAfterResponse(func(resp *httpclient.Response) error {
        log.Info("Response received",
            "status", resp.StatusCode,
            "duration", resp.Duration,
        )
        return nil
    }),
)
```

### 4. 自定义 Transport

```go
// 自定义 Transport（如跳过 TLS 验证）
client := httpclient.NewClient(
    httpclient.WithInsecureSkipVerify(), // 仅开发环境使用
)

// 或自定义完整 Transport
transport := &http.Transport{
    MaxIdleConns:        100,
    IdleConnTimeout:     90 * time.Second,
    DisableCompression:  true,
}

client := httpclient.NewClient(
    httpclient.WithTransport(transport),
)
```

---

## Retry 集成

### 1. 全局默认 Retry

```go
import "github.com/KOMKZ/go-yogan-framework/retry"

client := httpclient.NewClient(
    httpclient.WithBaseURL("https://api.example.com"),
    httpclient.WithRetry(
        retry.MaxAttempts(3),
        retry.Backoff(retry.ExponentialBackoff(time.Second)),
        retry.Condition(retry.RetryOnHTTPStatus(429, 503, 504)),
    ),
)

// 所有请求自动重试
resp, err := client.Get(ctx, "/users/123")
```

### 2. 请求级 Retry（覆盖全局）

```go
// 重要请求：更多重试
resp, err := client.Get(ctx, "/critical-api",
    httpclient.WithRetry(retry.MaxAttempts(5)),
)

// 非幂等操作：禁用重试
resp, err := client.Post(ctx, "/orders",
    httpclient.WithJSON(orderData),
    httpclient.DisableRetry(),
)
```

### 3. 使用预设策略

```go
// 使用 HTTP 默认策略
resp, err := client.Get(ctx, "/api/users",
    httpclient.WithRetryDefaults(),
)

// 自定义预设
var AggressiveRetry = httpclient.WithRetry(
    retry.MaxAttempts(10),
    retry.Backoff(retry.ExponentialBackoff(500*time.Millisecond)),
    retry.OnRetry(func(attempt int, err error) {
        log.Warn("Retrying", "attempt", attempt, "error", err)
    }),
)

resp, err := client.Get(ctx, "/api/important", AggressiveRetry)
```

### 4. Retry 与超时协同

```go
// Context Deadline 控制总时间
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

// 单次请求超时
resp, err := client.Get(ctx, "/api/users",
    httpclient.WithTimeout(2*time.Second),  // 单次最多 2s
    httpclient.WithRetry(retry.MaxAttempts(5)),  // 最多重试 5 次
)

// 执行逻辑：
// - 每次请求最多 2s
// - 最多重试 5 次
// - 总时间不超过 10s（Context Deadline）
// - retry 包会自动检测剩余时间，时间不足时停止重试
```

---

## Breaker 集成

### 1. 启用熔断器

```go
import "github.com/KOMKZ/go-yogan-framework/breaker"

// 1. 创建熔断器管理器
breakerConfig := breaker.DefaultConfig()
breakerConfig.Enabled = true
breakerConfig.Default = breaker.ResourceConfig{
    Strategy:           "error_rate",
    ErrorRateThreshold: 0.5,  // 错误率 50%
    Timeout:            30 * time.Second,
}

breakerMgr, _ := breaker.NewManager(breakerConfig)

// 2. 创建 HTTP Client（全局启用熔断器）
client := httpclient.NewClient(
    httpclient.WithBaseURL("https://api.example.com"),
    httpclient.WithBreaker(breakerMgr),
)

// 3. 正常使用（自动熔断保护）
resp, err := client.Get(ctx, "/users/123")
```

### 2. 自定义资源名称

```go
// 按服务分组熔断
resp, err := client.Get(ctx, "/users/123",
    httpclient.WithBreakerResource("user-service"),
)

resp, err := client.Get(ctx, "/orders/456",
    httpclient.WithBreakerResource("order-service"),
)
```

### 3. 降级逻辑

```go
// 设置降级逻辑（熔断时返回缓存）
fallback := func(ctx context.Context, err error) (*httpclient.Response, error) {
    log.Warn("Circuit breaker triggered, using cache", "error", err)
    
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

### 4. 禁用熔断器

```go
// 某些关键请求不希望被熔断
resp, err := client.Post(ctx, "/critical-operation",
    httpclient.WithJSON(data),
    httpclient.DisableBreaker(),
)
```

### 5. Breaker + Retry 协同

```go
// 执行顺序：Retry → Breaker → HTTP Request

client := httpclient.NewClient(
    httpclient.WithBreaker(breakerMgr),  // 熔断器
    httpclient.WithRetry(                // 重试
        retry.MaxAttempts(3),
        retry.Backoff(retry.ExponentialBackoff(time.Second)),
    ),
)

// 执行流程：
// 1. Retry 尝试第 1 次 → Breaker 检查 → HTTP 请求失败
// 2. Retry 尝试第 2 次 → Breaker 检查 → HTTP 请求失败
// 3. Retry 尝试第 3 次 → Breaker 检查 → HTTP 请求成功
// 4. 如果所有重试都失败，Breaker 记录失败并可能触发熔断

resp, err := client.Get(ctx, "/api/data")
```

---

## 完整示例

### 示例 1：用户服务客户端

```go
package client

import (
    "context"
    "fmt"
    "time"
    
    "github.com/KOMKZ/go-yogan-framework/httpclient"
    "github.com/KOMKZ/go-yogan-framework/retry"
    "github.com/KOMKZ/go-yogan-framework/breaker"
)

// User 用户模型
type User struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

// UserClient 用户服务客户端
type UserClient struct {
    client *httpclient.Client
}

// NewUserClient 创建用户客户端
func NewUserClient(baseURL string, breakerMgr *breaker.Manager) *UserClient {
    client := httpclient.NewClient(
        httpclient.WithBaseURL(baseURL),
        httpclient.WithTimeout(10*time.Second),
        httpclient.WithHeader("User-Agent", "UserService/1.0"),
        
        // 启用重试
        httpclient.WithRetry(
            retry.MaxAttempts(3),
            retry.Backoff(retry.ExponentialBackoff(time.Second)),
            retry.Condition(retry.RetryOnHTTPStatus(429, 503, 504)),
        ),
        
        // 启用熔断器
        httpclient.WithBreaker(breakerMgr),
        httpclient.WithBreakerResource("user-service"),
    )
    
    return &UserClient{client: client}
}

// GetUser 获取用户（幂等，可重试）
func (c *UserClient) GetUser(ctx context.Context, id int) (*User, error) {
    return httpclient.Get[User](c.client, ctx, fmt.Sprintf("/users/%d", id))
}

// CreateUser 创建用户（非幂等，禁用重试）
func (c *UserClient) CreateUser(ctx context.Context, user *User) (*User, error) {
    req := httpclient.NewPostRequest("/users")
    req.WithJSON(user)
    
    return httpclient.DoWithData[User](c.client, ctx, req,
        httpclient.DisableRetry(), // 禁用重试
    )
}

// UpdateUser 更新用户（幂等，可重试）
func (c *UserClient) UpdateUser(ctx context.Context, id int, user *User) (*User, error) {
    return httpclient.Put[User](c.client, ctx, fmt.Sprintf("/users/%d", id), user)
}

// DeleteUser 删除用户（幂等，可重试）
func (c *UserClient) DeleteUser(ctx context.Context, id int) error {
    resp, err := c.client.Delete(ctx, fmt.Sprintf("/users/%d", id))
    if err != nil {
        return err
    }
    defer resp.Close()
    
    if !resp.IsSuccess() {
        return fmt.Errorf("delete failed: %d %s", resp.StatusCode, resp.Status)
    }
    
    return nil
}

// ListUsers 列表查询（分页）
func (c *UserClient) ListUsers(ctx context.Context, page, limit int) ([]*User, error) {
    return httpclient.Get[[]*User](c.client, ctx, "/users",
        httpclient.WithQuery("page", fmt.Sprint(page)),
        httpclient.WithQuery("limit", fmt.Sprint(limit)),
    )
}
```

### 示例 2：支付服务客户端（带降级）

```go
package client

import (
    "context"
    "fmt"
    "time"
    
    "github.com/KOMKZ/go-yogan-framework/httpclient"
    "github.com/KOMKZ/go-yogan-framework/retry"
    "github.com/KOMKZ/go-yogan-framework/breaker"
)

type PaymentClient struct {
    client *httpclient.Client
}

func NewPaymentClient(baseURL string, breakerMgr *breaker.Manager) *PaymentClient {
    // 降级逻辑：返回支付挂起状态
    fallback := func(ctx context.Context, err error) (*httpclient.Response, error) {
        log.Warn("Payment service unavailable, using fallback", "error", err)
        
        // 返回默认响应（支付挂起）
        return &httpclient.Response{
            StatusCode: 200,
            Body:       []byte(`{"status": "pending", "message": "Payment service temporarily unavailable"}`),
        }, nil
    }
    
    client := httpclient.NewClient(
        httpclient.WithBaseURL(baseURL),
        httpclient.WithTimeout(30*time.Second), // 支付超时时间长
        
        // 支付服务：更严格的重试策略
        httpclient.WithRetry(
            retry.MaxAttempts(5),
            retry.Backoff(retry.ExponentialBackoff(2*time.Second)),
            retry.Condition(retry.RetryOnHTTPStatus(503, 504)), // 只重试服务错误
        ),
        
        // 支付服务：更严格的熔断策略
        httpclient.WithBreaker(breakerMgr),
        httpclient.WithBreakerResource("payment-service"),
        httpclient.WithBreakerFallback(fallback),
    )
    
    return &PaymentClient{client: client}
}

// CreatePayment 创建支付（使用幂等键）
func (c *PaymentClient) CreatePayment(ctx context.Context, req *PaymentRequest) (*PaymentResponse, error) {
    httpReq := httpclient.NewPostRequest("/payments")
    httpReq.WithJSON(req)
    httpReq.WithHeader("Idempotency-Key", req.IdempotencyKey) // 幂等键
    
    return httpclient.DoWithData[PaymentResponse](c.client, ctx, httpReq)
}
```

---

## 最佳实践

### 1. Client 管理

✅ **推荐**：创建全局单例 Client，复用连接池

```go
var apiClient = httpclient.NewClient(
    httpclient.WithBaseURL("https://api.example.com"),
    httpclient.WithTimeout(10*time.Second),
)
```

❌ **不推荐**：每次创建新 Client

```go
func GetUser(ctx context.Context, id int) (*User, error) {
    client := httpclient.NewClient()  // ❌ 每次创建，无法复用连接池
    return httpclient.Get[User](client, ctx, "/users/"+fmt.Sprint(id))
}
```

### 2. 重试策略

| 场景 | 策略 | 说明 |
|------|------|------|
| **幂等查询（GET）** | `MaxAttempts(3-5)` | 可以积极重试 |
| **非幂等写入（POST）** | `DisableRetry()` | 禁用重试或使用幂等键 |
| **有幂等键的写入** | `MaxAttempts(3)` | 可以安全重试 |
| **后台任务** | `MaxAttempts(10)` | 长时间重试 |
| **实时查询** | `MaxAttempts(2)` | 快速失败 |

### 3. 熔断器资源分组

✅ **推荐**：按服务分组

```go
client.Get(ctx, "/users/123",
    httpclient.WithBreakerResource("user-service"),
)
client.Get(ctx, "/users/456",
    httpclient.WithBreakerResource("user-service"),
)
```

❌ **不推荐**：每个 URL 独立资源

```go
client.Get(ctx, "/users/123")  // 资源: /users/123
client.Get(ctx, "/users/456")  // 资源: /users/456
```

### 4. 错误处理

```go
resp, err := client.Get(ctx, "/users/123")
if err != nil {
    // 网络错误或重试失败
    if errors.Is(err, context.DeadlineExceeded) {
        return ErrTimeout
    }
    return fmt.Errorf("request failed: %w", err)
}
defer resp.Close()

// HTTP 错误
if !resp.IsSuccess() {
    return fmt.Errorf("HTTP error: %d, body: %s", resp.StatusCode, resp.String())
}

// 反序列化
var user User
if err := resp.JSON(&user); err != nil {
    return fmt.Errorf("decode failed: %w", err)
}
```

### 5. 超时配置

```go
// 多层超时控制
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

resp, err := client.Get(ctx, "/users",
    httpclient.WithTimeout(5*time.Second),  // 单次请求超时
    httpclient.WithRetry(retry.MaxAttempts(5)),
)
// 总耗时不超过 30s，单次不超过 5s
```

---

## 相关文档

- 📖 [熔断器集成详解](./BREAKER.md)
- 📖 [Retry 工具库](../retry/README.md)
- 📖 [Breaker 组件](../breaker/README.md)
- 📖 [设计文档](../../../../articles/182-httpclient-design-analysis.md)

---

## 常见问题

### Q: 如何设置全局 Headers？

```go
client := httpclient.NewClient(
    httpclient.WithHeader("User-Agent", "MyApp/1.0"),
    httpclient.WithHeader("Accept", "application/json"),
)
```

### Q: 如何禁用 TLS 验证（仅开发环境）？

```go
client := httpclient.NewClient(
    httpclient.WithInsecureSkipVerify(),
)
```

### Q: 如何获取原始 http.Response？

```go
resp, err := client.Get(ctx, "/users/123")
rawResp := resp.RawResponse  // *http.Response
```

### Q: 熔断器和重试如何协同工作？

执行顺序：`Retry → Breaker → HTTP Request`

- Retry 在外层控制整体重试
- Breaker 在内层保护每次请求
- 如果熔断器打开，Retry 会立即收到错误并停止重试

### Q: 如何监控熔断器状态？

```go
eventBus := breakerMgr.GetEventBus()
eventBus.Subscribe(breaker.EventListenerFunc(func(event breaker.Event) {
    if event.Type() == breaker.EventStateChanged {
        log.Warn("Circuit breaker state changed",
            "resource", event.Resource(),
            "state", event.(*breaker.StateChangedEvent).ToState,
        )
    }
}))
```

---

**测试覆盖率**: 96.2%  
**GitHub**: [go-yogan-framework](https://github.com/KOMKZ/go-yogan-framework)

