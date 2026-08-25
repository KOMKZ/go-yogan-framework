# Limiter 配置说明

## 核心设计理念

### 配置驱动的限流策略

限流器采用**配置驱动**设计：
- ✅ **中间件全局应用**：作用于所有接口
- ✅ **配置驱动限流**：只对配置了的资源进行限流
- ✅ **default 自动生效**：配置了有效的 `default` 则自动应用到未配置资源
- ✅ **未配置自动放行**：如果 `default` 无效或未配置，未配置资源直接放行
- ✅ **按需启用**：通过配置精确控制哪些接口需要限流
- ✅ **规则级限流**：通过 `rules` 精确匹配 method+path，并支持 `path_ip`、`user_path`

### 独立配置文件

框架会自动加载 `rate_limiter.yaml`。优先级为：

`config.yaml` < `rate_limiter.yaml` < `{env}.yaml` < 环境变量 < flags。

### 工作流程

```
请求 → 中间件 → 检查资源是否配置 
                    ↓
        配置存在 → 执行限流检查 → 允许/拒绝
                    ↓
        配置不存在 → 检查 default 配置
                    ↓
        default 有效 → 使用 default 配置限流
                    ↓
        default 无效/未配置 → 自动放行
```

## 配置示例

```yaml
# 限流器配置
limiter:
  enabled: true                    # 是否启用限流器
  store_type: "memory"             # 存储类型：memory（单机）、redis（分布式）
  event_bus_buffer: 500            # 事件总线缓冲区大小
  
  # Redis 配置（store_type=redis 时有效）
  redis:
    instance: "main"               # Redis 实例名称
    key_prefix: "limiter:"         # Key 前缀
  
  # 🎯 默认配置（如果配置了有效的 default，自动应用到未配置资源；否则未配置资源直接放行）
  default:
    algorithm: "token_bucket"      # 限流算法
    rate: 100                      # 速率（tokens/s 或 reqs/s）
    capacity: 200                  # 容量
    init_tokens: 100               # 初始令牌数
  
	  # 资源级配置（覆盖默认配置）
	  resources:
	    "POST:/api/users":
	      algorithm: "token_bucket"
	      rate: 10
	      capacity: 20

	  # 规则级配置：按 method+path 匹配，适合登录、注册、用户高成本接口
	  rules:
	    login:
	      key_func: "path_ip"
	      match:
	        - method: "POST"
	          path: "/api/auth/login"
	      limit:
	        algorithm: "token_bucket"
	        rate: 1
	        capacity: 5
	        init_tokens: 5
	    user_profile:
	      key_func: "user_path"
	      identity_source: "jwt_context_or_token"
	      user_id_key: "user_id"
	      match:
	        - method: "GET"
	          path: "/api/user/profile"
	      limit:
	        algorithm: "token_bucket"
	        rate: 10
	        capacity: 20
	        init_tokens: 20

# Redis 实例配置：store_type=redis 时 limiter.redis.instance 必须指向这里
redis:
  instances:
    limiter:
      mode: "standalone"
      addrs: ["127.0.0.1:6379"]
      db: 7
```

## 配置项说明

### limiter（限流器核心配置）

#### 基本配置

| 配置项 | 类型 | 默认值 | 说明 |
|-------|------|--------|------|
| `enabled` | bool | true | 是否启用限流器 |
| `store_type` | string | memory | 存储类型：memory（单机内存）、redis（分布式Redis） |
| `event_bus_buffer` | int | 500 | 事件总线缓冲区大小 |

#### Redis 配置（store_type=redis 时）

| 配置项 | 类型 | 默认值 | 说明 |
|-------|------|--------|------|
| `redis.instance` | string | main | Redis 实例名称（需在 redis.instances 中配置） |
| `redis.key_prefix` | string | limiter: | Redis key 前缀 |

#### 默认限流配置（default）

**核心机制**：
- ✅ **配置了有效的 `default`**：自动应用到所有未在 `resources` 中配置的资源
- ✅ **`default` 无效或未配置**：未配置资源直接放行

**使用场景**：

1. **不配置 `default`（渐进式限流）**：
   ```yaml
   limiter:
     # default: {}  # 不配置或留空
     resources:
       "POST:/api/orders": { rate: 10 }  # 只限流特定接口
   ```
   - 效果：只有 `POST:/api/orders` 受限流，其他接口不限流

2. **配置有效的 `default`（全局保护）**：
   ```yaml
   limiter:
     default:
       algorithm: "token_bucket"
       rate: 100  # 默认 100 QPS
       capacity: 200
       init_tokens: 100
     resources:
       "GET:/api/health": { rate: 1000 }  # 健康检查放宽
   ```
   - 效果：所有接口默认 100 QPS，`/api/health` 放宽到 1000 QPS

| 配置项 | 类型 | 默认值 | 说明 |
|-------|------|--------|------|
| `algorithm` | string | token_bucket | 限流算法（见下方算法说明） |
| `rate` | int | 100 | 速率：令牌生成速率或请求速率 |
| `capacity` | int | 200 | 容量：令牌桶容量或窗口大小 |
| `init_tokens` | int | 100 | 初始令牌数（仅 token_bucket） |

#### 资源级配置（resources）

针对特定资源（如 `GET:/api/users`）的精确配置，优先级高于 default。

### middleware.rate_limit（中间件配置）

| 配置项 | 类型 | 默认值 | 说明 |
|-------|------|--------|------|
| `enable` | bool | true | 是否启用限流中间件 |
| `key_func` | string | path | 资源键生成方式（见下方说明） |
| `skip_paths` | []string | [] | 跳过限流的路径列表 |

#### key_func 说明

| 值 | 说明 | 资源键格式 | 使用场景 |
|---|------|-----------|---------|
| `path` | 按路径限流 | `get:/api/users` | 全局接口限流 |
| `ip` | 按IP限流 | `ip:192.168.1.1` | 防止单个IP滥用 |
| `user` | 按用户限流 | `user:12345` | 用户级别限流 |
| `path_ip` | 按路径+IP限流 | `get:/api/users:192.168.1.1` | 接口+IP双维度 |
| `user_path` | 按用户+路径限流 | `user:123:get:/api/users` | 登录态高成本接口 |
| `api_key` | 按API Key限流 | `apikey:xxx-xxx` | API服务限流 |

#### rules 说明

`rules` 用于精确保护部分接口。每个 rule 必须声明 `match` 和 `limit`。

| 配置项 | 类型 | 说明 |
|-------|------|------|
| `key_func` | string | 规则维度，支持 `path_ip`、`user_path` |
| `identity_source` | string | `user_path` 获取用户身份的来源，支持 `context`、`jwt_context_or_token` |
| `user_id_key` | string | Gin context 中的用户 ID key，默认 `user_id` |
| `match` | []object | 精确匹配的 HTTP method 和 path |
| `limit` | object | 当前规则使用的资源限流配置，可继承 `default` 未覆盖字段 |

## 限流算法说明

### 1. Token Bucket（令牌桶）- 推荐

**适用场景**：需要支持突发流量的场景

```yaml
algorithm: "token_bucket"
rate: 100          # 每秒生成100个令牌
capacity: 200      # 桶最多容纳200个令牌
init_tokens: 100   # 初始100个令牌
```

**特点**：
- ✅ 允许突发流量（capacity > rate）
- ✅ 平滑限流
- ✅ 性能优秀

### 2. Sliding Window（滑动窗口）

**适用场景**：需要精确QPS控制的场景

```yaml
algorithm: "sliding_window"
limit: 1000              # 限制数量
window_size: 60s         # 时间窗口
bucket_size: 1s          # 桶大小（可选）
```

**特点**：
- ✅ 精确QPS控制
- ✅ 防止突发流量
- ❌ 内存占用较高

### 3. Concurrency（并发限流）

**适用场景**：控制并发数的场景（如数据库连接、文件下载）

```yaml
algorithm: "concurrency"
max_concurrency: 10      # 最大并发数
```

**特点**：
- ✅ 控制资源并发
- ✅ 内存占用低
- ⚠️ 需手动释放（Release）

### 4. Adaptive（自适应限流）

**适用场景**：根据系统负载动态调整限流的场景

```yaml
algorithm: "adaptive"
min_limit: 10           # 最小限流值
max_limit: 100          # 最大限流值
target_cpu: 0.7         # 目标CPU 70%
target_memory: 0.8      # 目标内存 80%
```

**特点**：
- ✅ 自动调整
- ✅ 保护系统
- ⚠️ 需要注入 AdaptiveProvider

## 配置示例

### 示例1：基本API限流

```yaml
limiter:
  enabled: true
  store_type: "memory"
  default:
    algorithm: "token_bucket"
    rate: 100
    capacity: 200
  resources:
    "POST:/api/users":
      algorithm: "token_bucket"
      rate: 10
      capacity: 20
```

### 示例2：按IP限流

```yaml
middleware:
  rate_limit:
    enable: true
    key_func: "ip"          # 按IP限流
    skip_paths:
      - "/health"

limiter:
  enabled: true
  default:
    algorithm: "token_bucket"
    rate: 1000              # 每个IP每秒1000请求
    capacity: 2000
```

### 示例3：分布式限流（Redis）

```yaml
limiter:
  enabled: true
  store_type: "redis"       # 使用Redis
  redis:
    instance: "main"
    key_prefix: "app:limiter:"
  default:
    algorithm: "token_bucket"
    rate: 100
    capacity: 200
```

### 示例4：多算法组合

```yaml
limiter:
  enabled: true
  resources:
    # 创建接口：令牌桶，允许突发
    "POST:/api/users":
      algorithm: "token_bucket"
      rate: 10
      capacity: 20
    
    # 查询接口：滑动窗口，精确控制
    "GET:/api/users":
      algorithm: "sliding_window"
      limit: 1000
      window_size: 60s
    
    # 下载接口：并发限流
    "GET:/api/download":
      algorithm: "concurrency"
      max_concurrency: 10
    
    # 重负载接口：自适应
    "POST:/api/heavy":
      algorithm: "adaptive"
      min_limit: 10
      max_limit: 100
      target_cpu: 0.7
```

## 最佳实践

### 1. 选择合适的算法

- **API接口**：Token Bucket（支持突发）
- **精确QPS**：Sliding Window
- **资源控制**：Concurrency
- **动态调整**：Adaptive

### 2. 选择合适的 key_func

- **全局限流**：`path`
- **防滥用**：`ip` 或 `api_key`
- **用户级别**：`user`
- **双重保护**：`path_ip`

### 3. 单机 vs 分布式

- **单机应用**：`store_type: memory`
- **多实例部署**：`store_type: redis`
- **高性能要求**：`store_type: memory`（单机性能更好）
- **全局限流**：`store_type: redis`（跨实例共享）

### 4. 配置合理的限流值

```yaml
# 保守配置（推荐）
rate: 100
capacity: 200  # 2倍突发

# 宽松配置
rate: 1000
capacity: 2000

# 严格配置
rate: 10
capacity: 10  # 不允许突发
```

### 5. 设置白名单

```yaml
middleware:
  rate_limit:
    skip_paths:
      - "/health"      # 健康检查
      - "/metrics"     # 监控指标
      - "/"            # 首页
```

## 监控和调试

### 1. 查看限流指标

```go
metrics := limiterManager.GetMetrics("GET:/api/users")
fmt.Printf("Current: %d, Limit: %d\n", metrics.Current, metrics.Limit)
```

### 2. 订阅限流事件

```go
eventBus := limiterManager.GetEventBus()
eventBus.Subscribe(func(e limiter.Event) {
    if e.Type() == limiter.EventRejected {
        log.Warn("请求被限流", zap.String("resource", e.Resource()))
    }
})
```

### 3. 日志输出

限流器会自动记录关键事件：
- 限流器初始化
- 资源首次访问
- 限流触发
- 配置变更

## 故障排查

### 问题1：限流器未生效

**检查清单**：
1. ✅ `limiter.enabled: true`
2. ✅ `middleware.rate_limit.enable: true`
3. ✅ 资源键是否匹配（查看日志）
4. ✅ 限流值是否合理

### 问题2：所有请求被限流

**可能原因**：
1. `init_tokens: 0`（初始无令牌）
2. `rate` 配置过小
3. `capacity` 配置过小

**解决方案**：
```yaml
init_tokens: 100  # 设置初始令牌
rate: 100         # 提高速率
capacity: 200     # 提高容量
```

### 问题3：Redis连接失败

**检查清单**：
1. ✅ Redis 实例是否配置
2. ✅ Redis 是否可访问
3. ✅ `redis.instance` 名称是否正确

## 性能优化

### 1. 内存优化

```yaml
# 减少事件总线缓冲
event_bus_buffer: 100

# 使用并发限流（内存占用低）
algorithm: "concurrency"
```

### 2. 性能优化

```yaml
# 使用内存存储（单机）
store_type: "memory"

# 使用令牌桶（性能最优）
algorithm: "token_bucket"
```

### 3. 分布式优化

```yaml
# 使用Redis（多实例）
store_type: "redis"

# 配置Redis连接池
redis:
  instances:
    main:
      pool_size: 20
      min_idle_conns: 10
```
