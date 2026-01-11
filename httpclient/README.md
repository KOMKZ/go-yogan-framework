# HTTPClient - 统一的 HTTP 客户端封装

Go-Yogan 框架的 HTTP 客户端工具库，基于函数式选项模式设计，天然集成 retry 工具库和熔断器。

> 📖 **详细使用指南**: 请查看 [USAGE.md](./USAGE.md) 获取完整的使用说明和示例

## ✨ 核心特性

- ✅ **函数式选项模式** - 灵活、可扩展、符合 Go 习惯
- ✅ **Request/Response 显式传递** - 细粒度控制，可复用
- ✅ **天然 Retry 集成** - 无缝集成 retry 工具库
- ✅ **熔断器支持** - 可插拔的熔断器集成，自动故障隔离
- ✅ **泛型支持** - Get[T] 自动反序列化，类型安全
- ✅ **多层配置** - Client 级 + Request 级，灵活覆盖
- ✅ **测试覆盖率 96.2%** - 高质量保证

## 📦 安装

```bash
go get github.com/KOMKZ/go-yogan-framework/httpclient
```

## 🚀 快速开始

### 基础用法

```go
package main

import (
    "context"
    "fmt"
    
    "github.com/KOMKZ/go-yogan-framework/httpclient"
)

func main() {
    // 创建全局 Client
    client := httpclient.NewClient(
        httpclient.WithBaseURL("https://api.example.com"),
        httpclient.WithTimeout(10*time.Second),
    )
    
    // 简单 GET 请求
    resp, err := client.Get(context.Background(), "/users/123")
    if err != nil {
        panic(err)
    }
    defer resp.Close()
    
    fmt.Println(resp.String())
}
```

### 泛型自动反序列化

```go
type User struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}

// 自动反序列化
user, err := httpclient.Get[User](client, ctx, "/users/123")
if err != nil {
    return err
}

fmt.Printf("User: %s <%s>\n", user.Name, user.Email)
```

### Request 显式传递

```go
// 构建可复用的 Request
req := httpclient.NewPostRequest("/users")
req.WithHeader("Authorization", "Bearer token")
req.WithJSON(map[string]string{"name": "Alice"})

// 执行（Request 可复用）
resp, err := client.Do(ctx, req)
```

### Retry 集成

```go
// 全局默认 Retry
client := httpclient.NewClient(
    httpclient.WithRetry(
        retry.MaxAttempts(3),
        retry.Backoff(retry.ExponentialBackoff(time.Second)),
    ),
)

// 请求级 Retry（覆盖全局）
resp, err := client.Get(ctx, "/critical-api",
    httpclient.WithRetry(retry.MaxAttempts(5)),
)

// 禁用 Retry
resp, err := client.Post(ctx, "/orders",
    httpclient.WithJSON(orderData),
    httpclient.DisableRetry(), // 非幂等操作
)
```

### Options 复用与组合

```go
// 定义可复用的 Options
var (
    WithAuth = httpclient.WithHeader("Authorization", "Bearer "+token)
    
    StandardRetry = httpclient.WithRetry(
        retry.MaxAttempts(3),
        retry.Backoff(retry.ExponentialBackoff(time.Second)),
    )
)

// 组合使用
resp, err := client.Get(ctx, "/api/users",
    WithAuth,
    StandardRetry,
    httpclient.WithQuery("page", "1"),
)
```

## 📚 API 文档

### Client 创建

```go
client := httpclient.NewClient(options...)
```

**Client 级别选项**:
- `WithBaseURL(url)` - 设置基础 URL
- `WithTimeout(duration)` - 设置超时时间
- `WithHeader(key, value)` - 设置单个 Header
- `WithHeaders(headers)` - 设置多个 Headers
- `WithTransport(transport)` - 设置自定义 Transport
- `WithCookieJar(jar)` - 设置 Cookie Jar
- `WithInsecureSkipVerify()` - 跳过 TLS 验证（仅开发环境）

### 基础方法

```go
// 执行请求
func (c *Client) Do(ctx context.Context, req *Request, opts ...Option) (*Response, error)

// HTTP 方法
func (c *Client) Get(ctx context.Context, url string, opts ...Option) (*Response, error)
func (c *Client) Post(ctx context.Context, url string, opts ...Option) (*Response, error)
func (c *Client) Put(ctx context.Context, url string, opts ...Option) (*Response, error)
func (c *Client) Delete(ctx context.Context, url string, opts ...Option) (*Response, error)
```

### 泛型方法

```go
// 自动反序列化
func Get[T any](client *Client, ctx context.Context, url string, opts ...Option) (*T, error)
func Post[T any](client *Client, ctx context.Context, url string, data interface{}, opts ...Option) (*T, error)
func Put[T any](client *Client, ctx context.Context, url string, data interface{}, opts ...Option) (*T, error)
func DoWithData[T any](client *Client, ctx context.Context, req *Request, opts ...Option) (*T, error)
```

### Request 级别选项

- `WithQuery(key, value)` - 设置单个 Query 参数
- `WithQueries(queries)` - 设置多个 Query 参数
- `WithBody(reader)` - 设置原始 Body
- `WithBodyString(s)` - 设置字符串 Body
- `WithJSON(data)` - 设置 JSON Body（自动序列化）
- `WithForm(data)` - 设置 Form Body
- `WithContext(ctx)` - 设置 Context
- `WithBeforeRequest(fn)` - 设置请求前钩子
- `WithAfterResponse(fn)` - 设置响应后钩子

### Retry 选项

- `WithRetry(opts...)` - 设置重试选项
- `WithRetryDefaults()` - 使用默认重试策略
- `DisableRetry()` - 禁用重试

### Breaker 选项

- `WithBreaker(manager)` - 设置熔断器管理器
- `WithBreakerResource(resource)` - 设置熔断器资源名称
- `WithBreakerFallback(fn)` - 设置熔断降级逻辑
- `DisableBreaker()` - 禁用熔断器

详见：[熔断器集成文档](./BREAKER.md)

### Request 构造

```go
req := httpclient.NewRequest(method, url)
req := httpclient.NewGetRequest(url)
req := httpclient.NewPostRequest(url)
req := httpclient.NewPutRequest(url)
req := httpclient.NewDeleteRequest(url)

// Request 方法（链式调用）
req.WithHeader(key, value)
req.WithQuery(key, value)
req.WithBody(reader)
req.WithJSON(data)
req.WithForm(data)
req.Clone() // 克隆 Request（用于重试）
```

### Response 方法

```go
resp.IsSuccess() bool        // 判断 2xx
resp.IsClientError() bool    // 判断 4xx
resp.IsServerError() bool    // 判断 5xx
resp.JSON(v interface{}) error  // 反序列化 JSON
resp.String() string         // 返回字符串
resp.Bytes() []byte          // 返回字节数组
resp.Close() error           // 关闭响应
```

## 🎯 使用场景

### 场景一：简单查询

```go
// 最简单的用法
user, err := httpclient.Get[User](client, ctx, "/users/123")

// 带查询参数
users, err := httpclient.Get[[]User](client, ctx, "/users",
    httpclient.WithQuery("page", "1"),
    httpclient.WithQuery("limit", "20"),
)
```

### 场景二：创建资源（非幂等）

```go
// 非幂等操作，禁用重试
resp, err := client.Post(ctx, "/orders",
    httpclient.WithJSON(orderData),
    httpclient.DisableRetry(),
)

// 或使用幂等键保障安全
resp, err := client.Post(ctx, "/orders",
    httpclient.WithJSON(orderData),
    httpclient.WithHeader("Idempotency-Key", uuid.New().String()),
    httpclient.WithRetry(retry.MaxAttempts(3)),
)
```

### 场景三：复杂业务流程

```go
func ProcessOrder(ctx context.Context, orderID int) error {
    // 1. 查询订单（幂等，可重试）
    order, err := client.Get[Order](ctx, fmt.Sprintf("/orders/%d", orderID),
        httpclient.WithRetry(retry.HTTPDefaults...),
    )
    if err != nil {
        return fmt.Errorf("fetch order failed: %w", err)
    }
    
    // 2. 调用支付接口（非幂等，慎重重试）
    req := httpclient.NewPostRequest("/payments")
    req.WithJSON(map[string]interface{}{
        "order_id": orderID,
        "amount":   order.Amount,
    })
    req.WithHeader("Idempotency-Key", order.PaymentKey)
    
    resp, err := client.Do(ctx, req,
        httpclient.WithTimeout(30*time.Second),
        httpclient.WithRetry(
            retry.MaxAttempts(3),
            retry.Backoff(retry.ExponentialBackoff(2*time.Second)),
            retry.Condition(retry.RetryOnHTTPStatus(503, 504)),
        ),
    )
    if err != nil {
        return fmt.Errorf("payment failed: %w", err)
    }
    defer resp.Close()
    
    // 3. 更新订单状态（幂等，可重试）
    _, err = client.Put(ctx, fmt.Sprintf("/orders/%d/status", orderID),
        httpclient.WithJSON(map[string]string{"status": "paid"}),
        httpclient.WithRetry(retry.MaxAttempts(5)),
    )
    
    return err
}
```

## 📊 测试覆盖率

```
$ go test -cover .
ok      github.com/KOMKZ/go-yogan-framework/httpclient  0.677s  coverage: 96.2% of statements
```

**代码统计**:
- 生产代码: 951 行 (含熔断器集成)
- 测试代码: 2668 行
- 测试/代码比: 2.8:1

**功能覆盖**:
- ✅ Request/Response 封装
- ✅ Client 核心方法
- ✅ Options 配置系统
- ✅ Retry 集成
- ✅ Breaker 集成 (新增)
- ✅ 泛型方法

## 🔗 相关文档

- [设计文档](../../../../articles/182-httpclient-design-analysis.md)
- [熔断器集成](./BREAKER.md) ⭐ 新增
- [Retry 工具库](../retry/README.md)
- [Breaker 组件](../breaker/README.md)
- [Go-Yogan 框架](../README.md)

## 📝 License

MIT License

