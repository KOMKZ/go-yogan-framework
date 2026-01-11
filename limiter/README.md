# Limiter - 限流器组件

高性能、可扩展的限流器组件，支持多种限流算法和存储方式。

## 特性

- ✅ **多种限流算法**
  - 令牌桶（Token Bucket）：支持突发流量
  - 滑动窗口（Sliding Window）：精确QPS控制
  - 并发限流（Concurrency）：控制并发数
  - 自适应限流（Adaptive）：根据系统负载动态调整

- ✅ **多种存储方式**
  - 内存存储：单机高性能
  - **Redis存储**：分布式共享（支持单机和集群）

- ✅ **事件驱动**
  - 可订阅限流事件（允许/拒绝/等待）
  - 实时指标采集
  - 支持自定义监听器

- ✅ **可选启用**
  - 配置驱动的限流策略
  - 未配置的资源自动放行
  - 优雅降级

- ✅ **依赖注入**
  - Logger注入
  - AdaptiveProvider可选注入

## 快速开始

### 基本使用

```go
package main

import (
    "context"
    "github.com/KOMKZ/go-yogan/limiter"
    "github.com/KOMKZ/go-yogan/logger"
)

func main() {
    // 1. 创建配置
    cfg := limiter.Config{
        Enabled:   true,
        StoreType: "memory",
        Default: limiter.ResourceConfig{
            Algorithm: "token_bucket",
            Rate:      100,  // 100 QPS
            Capacity:  200,  // 允许200突发
        },
    }
    
    // 2. 创建限流器
    log := logger.NewCtxZapLogger("app")
    lim, _ := limiter.NewManagerWithLogger(cfg, log, nil)
    defer lim.Close()
    
    // 3. 使用限流器
    ctx := context.Background()
    allowed, _ := lim.Allow(ctx, "api:/users")
    
    if !allowed {
        // 请求被限流
        return
    }
    
    // 处理请求
}
```

### Gin中间件集成（推荐方式）

**设计理念**：
- **中间件全局应用**：作用于所有接口
- **配置驱动限流**：只对配置了的资源进行限流
- **未配置自动放行**：未在 `limiter.resources` 中配置的接口不受限流影响

```go
import (
    "github.com/KOMKZ/go-yogan/limiter"
    "github.com/KOMKZ/go-yogan/middleware"
    "github.com/gin-gonic/gin"
)

func main() {
    // 1. 创建限流器，配置需要限流的资源
    cfg := limiter.Config{
        Enabled:   true,
        StoreType: "memory",
        Resources: map[string]limiter.ResourceConfig{
            // 只对这些资源进行限流
            "POST:/api/users": {
                Algorithm: "token_bucket",
                Rate:      10,   // 10 req/s
                Capacity:  20,
            },
            "GET:/limiter-test": {
                Algorithm: "token_bucket",
                Rate:      5,
                Capacity:  10,
            },
        },
    }
    
    log := logger.NewCtxZapLogger("app")
    manager, _ := limiter.NewManagerWithLogger(cfg, log, nil)
    
    // 2. 创建Gin应用
    router := gin.Default()
    
    // 3. 全局应用限流中间件（推荐）
    //    - 中间件作用于所有接口
    //    - 但只对在 Resources 中配置的资源进行限流
    //    - 未配置的资源会自动放行
    rateLimiterCfg := middleware.DefaultRateLimiterConfig(manager)
    rateLimiterCfg.KeyFunc = middleware.RateLimiterKeyByPath  // 按路径限流
    rateLimiterCfg.SkipPaths = []string{"/", "/health"}       // 白名单（跳过中间件）
    
    router.Use(middleware.RateLimiterWithConfig(rateLimiterCfg))
    
    // 4. 注册路由
    router.POST("/api/users", createUser)      // ✅ 会被限流（已配置）
    router.GET("/api/users", getUsers)         // ✅ 不会被限流（未配置，自动放行）
    router.GET("/limiter-test", limiterTest)   // ✅ 会被限流（已配置）
    router.GET("/", index)                     // ✅ 不会被限流（在白名单中）
    
    router.Run(":8080")
}

// 其他键函数示例
// rateLimiterCfg.KeyFunc = middleware.RateLimiterKeyByIP           // 按IP限流
// rateLimiterCfg.KeyFunc = middleware.RateLimiterKeyByUser         // 按用户限流
// rateLimiterCfg.KeyFunc = middleware.RateLimiterKeyByPathAndIP    // 按路径+IP限流
// rateLimiterCfg.KeyFunc = middleware.RateLimiterKeyByAPIKey       // 按API Key限流
```
// - middleware.RateLimiterKeyByAPIKey       // 按API Key限流
```

### 配置示例

```yaml
limiter:
  enabled: true
  store_type: memory
  event_bus_buffer: 500
  
  # 默认配置
  default:
    algorithm: token_bucket
    rate: 100
    capacity: 200
  
  # 资源级配置
  resources:
    # 令牌桶
    "POST:/api/users":
      algorithm: token_bucket
      rate: 50
      capacity: 100
    
    # 滑动窗口
    "GET:/api/orders":
      algorithm: sliding_window
      limit: 1000
      window_size: 1s
    
    # 并发限流
    "db:query":
      algorithm: concurrency
      max_concurrency: 50
    
    # 自适应限流
    "grpc:OrderService":
      algorithm: adaptive
      min_limit: 100
      max_limit: 1000
      target_cpu: 0.7
```

## 测试

```bash
cd src/yogan/limiter
go test -v -cover
```

## 测试覆盖率

当前覆盖率：**82.6%**

已完成核心功能测试：
- ✅ 令牌桶算法（完整测试）
- ✅ 滑动窗口算法（完整测试）
- ✅ 并发限流算法（完整测试）
- ✅ 自适应限流算法（完整测试）
- ✅ 内存存储（完整测试）
- ✅ **Redis存储**（完整测试，使用 miniredis）
- ✅ 配置管理（完整测试）
- ✅ 限流器Manager（完整测试）
- ✅ 事件系统（完整测试）

## 架构设计

参考：`articles/132-rate-limiter-architecture-design.md`

### 核心组件

```
├── limiter.go              # 核心接口
├── limiter_impl.go         # Manager实现
├── algorithm.go            # 算法接口
├── algo_token_bucket.go    # 令牌桶算法
├── algo_sliding_window.go  # 滑动窗口算法
├── algo_concurrency.go     # 并发限流算法
├── algo_adaptive.go        # 自适应限流算法
├── store.go                # 存储接口
├── store_memory.go         # 内存存储
├── config.go               # 配置管理
├── event.go                # 事件定义
├── event_bus.go            # 事件总线
├── metrics.go              # 指标采集
└── errors.go               # 错误定义
```

### 设计原则

- **SOLID**: 单一职责、开闭原则、里氏替换、接口隔离、依赖倒置
- **DRY**: 公共逻辑提取复用
- **KISS**: API简洁直观
- **YAGNI**: 不过度设计

### 策略模式

- 算法可插拔（Token Bucket/Sliding Window/Concurrency/Adaptive）
- 存储可切换（Memory/Redis）
- 事件驱动（可观测、可扩展）

## TODO

- [ ] 提升测试覆盖率到95%+
- [x] **Gin中间件**（已完成，已迁移到 middleware 包）
- [ ] gRPC拦截器
- [ ] 集成到Governance组件

## License

MIT

