# Middleware - Yogan 中间件组件

高质量、可配置的 Gin 中间件集合。

## 特性

- ✅ **TraceID** - 请求追踪
- ✅ **RequestLog** - 结构化请求日志
- ✅ **Recovery** - Panic 恢复
- ✅ **CORS** - 跨域支持
- ✅ **RateLimiter** - 限流控制 ✨

## RateLimiter - 限流中间件

### 特性

- 支持多种限流算法（Token Bucket、Sliding Window、Concurrency、Adaptive）
- 支持多种限流维度（路径、IP、用户、API Key）
- 支持路径跳过和自定义跳过条件
- 限流器未启用时自动放行
- 限流器错误时降级放行（高可用）
- 完全可配置（错误处理、限流响应、键函数）

### 基本使用

```go
import (
    "github.com/KOMKZ/go-yogan/limiter"
    "github.com/KOMKZ/go-yogan/middleware"
    "github.com/gin-gonic/gin"
)

// 1. 创建限流器
cfg := limiter.Config{
    Enabled:   true,
    StoreType: "memory",
    Resources: map[string]limiter.ResourceConfig{
        "GET:/api/users": {
            Algorithm: "token_bucket",
            Rate:      100,
            Capacity:  200,
        },
    },
}

manager, _ := limiter.NewManagerWithLogger(cfg, log, nil)

// 2. 应用中间件
router := gin.Default()
router.Use(middleware.RateLimiter(manager))
```

### 自定义配置

```go
// 配置限流中间件
cfg := middleware.DefaultRateLimiterConfig(manager)

// 按IP限流
cfg.KeyFunc = middleware.RateLimiterKeyByIP

// 跳过特定路径
cfg.SkipPaths = []string{"/health", "/metrics"}

// 自定义限流响应
cfg.RateLimitHandler = func(c *gin.Context) {
    c.JSON(429, gin.H{
        "error": "Too many requests",
        "retry_after": 60,
    })
    c.Abort()
}

// 自定义跳过条件
cfg.SkipFunc = func(c *gin.Context) bool {
    // 管理员不限流
    role, _ := c.Get("role")
    return role == "admin"
}

router.Use(middleware.RateLimiterWithConfig(cfg))
```

### 内置键函数

```go
// 1. 默认：按路径限流
// 资源键：GET:/api/users
cfg.KeyFunc = nil  // 或不设置

// 2. 按IP限流
// 资源键：ip:192.168.1.1
cfg.KeyFunc = middleware.RateLimiterKeyByIP

// 3. 按用户限流
// 资源键：user:12345 或 user:anonymous
cfg.KeyFunc = middleware.RateLimiterKeyByUser("user_id")

// 4. 按路径+IP限流
// 资源键：GET:/api/users:192.168.1.1
cfg.KeyFunc = middleware.RateLimiterKeyByPathAndIP

// 5. 按API Key限流
// 资源键：apikey:xxx-xxx 或 apikey:anonymous
cfg.KeyFunc = middleware.RateLimiterKeyByAPIKey("X-API-Key")

// 6. 自定义键函数
cfg.KeyFunc = func(c *gin.Context) string {
    // 按租户ID限流
    tenantID := c.GetHeader("X-Tenant-ID")
    return fmt.Sprintf("tenant:%s:%s", tenantID, c.Request.URL.Path)
}
```

### 路由级限流

```go
// 全局限流（所有路由）
router.Use(middleware.RateLimiter(manager))

// 路由组限流
api := router.Group("/api")
api.Use(middleware.RateLimiter(manager))
{
    api.GET("/users", getUsersHandler)
    api.POST("/users", createUserHandler)
}

// 单个路由限流
router.GET("/download",
    middleware.RateLimiter(manager),
    downloadHandler,
)
```

### 配置示例

#### 按路径限流（默认）
```yaml
limiter:
  enabled: true
  store_type: memory
  resources:
    "POST:/api/users":
      algorithm: token_bucket
      rate: 10        # 10 req/s
      capacity: 20
    "GET:/api/users":
      algorithm: sliding_window
      limit: 1000
      window_size: 1m
```

#### 按IP限流
```go
cfg.KeyFunc = middleware.RateLimiterKeyByIP
cfg.Resources = map[string]limiter.ResourceConfig{
    "ip:*": {  // 所有IP共用配置
        Algorithm: "token_bucket",
        Rate:      100,
        Capacity:  200,
    },
}
```

#### 按用户限流
```go
// 先设置用户上下文
router.Use(authMiddleware)  // 设置 c.Set("user_id", "xxx")

// 配置限流
cfg.KeyFunc = middleware.RateLimiterKeyByUser("user_id")
cfg.Resources = map[string]limiter.ResourceConfig{
    "user:*": {
        Algorithm: "token_bucket",
        Rate:      50,
        Capacity:  100,
    },
}
```

### 高级功能

#### 1. 分布式限流（Redis）
```go
// 使用Redis存储实现分布式限流
cfg := limiter.Config{
    Enabled:   true,
    StoreType: "redis",
    RedisConfig: limiter.RedisStoreConfig{
        Instance: "main",  // 使用的Redis实例名
    },
}
```

#### 2. 自适应限流
```go
// 根据系统负载动态调整限流
cfg.Resources = map[string]limiter.ResourceConfig{
    "POST:/api/heavy": {
        Algorithm: "adaptive",
        MinLimit:  10,
        MaxLimit:  100,
        TargetCPU: 0.7,  // 目标CPU 70%
    },
}
```

#### 3. 并发限流
```go
// 控制并发请求数
cfg.Resources = map[string]limiter.ResourceConfig{
    "GET:/api/export": {
        Algorithm:      "concurrency",
        MaxConcurrency: 10,  // 最多10个并发
    },
}
```

### 测试

```bash
cd src/yogan/middleware
go test -v -run='TestRateLimiter'
```

### 监控

限流中间件会发送事件到事件总线，可以订阅这些事件进行监控：

```go
eventBus := manager.GetEventBus()
eventBus.Subscribe(func(e limiter.Event) {
    if e.Type() == limiter.EventRejected {
        log.Warn("请求被限流", 
            zap.String("resource", e.Resource()),
        )
    }
})
```

## 其他中间件

### TraceID
```go
router.Use(middleware.TraceID(middleware.DefaultTraceConfig()))
```

### RequestLog
```go
cfg := middleware.DefaultRequestLogConfig()
cfg.SkipPaths = []string{"/health"}
router.Use(middleware.RequestLogWithConfig(cfg))
```

### Recovery
```go
router.Use(middleware.Recovery())
```

### CORS
```go
router.Use(middleware.CORS())
```

## License

MIT

