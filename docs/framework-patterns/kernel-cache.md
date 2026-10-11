# 缓存

本页覆盖两块能力：业务缓存策略目录（PolicyCatalog）与事件驱动失效。

## 业务缓存策略目录（PolicyCatalog）

`cache.PolicyCatalog` 解决"一个 app 内多个业务缓存场景如何统一管理开关与 TTL"的问题：
配置由 app 启动时一次性解析，framework 只提供通用 definition/policy 结构，不知道任何
业务配置根 key，也不引入 Viper。

### 分层契约

| 层 | 职责 |
|---|---|
| app 配置 provider | 从自身 YAML `UnmarshalKey` 一次，构造 `PolicyCatalog` 并注册 DI |
| `cache.PolicyCatalog` | 合并全局/领域/场景三级开关，校验 TTL，构造后不可变 |
| 领域 provider/do | 用 domain/scenario 常量调用一次 `Resolve`，把 `Policy` 交给领域 cache |
| 领域 cache | 拥有 key、序列化、cache-aside、singleflight、降级和 TTL 执行 |

### 使用方式

```go
// app 侧：启动时唯一入口构造目录（定义非法直接启动失败）
catalog, err := cache.NewPolicyCatalog(def)

// 领域 provider 侧：常量解析，声明缺失 fail-fast
policy, err := catalog.Resolve("my-domain", "my-scenario")
// policy.Enabled / policy.TTL / policy.RedisInstance
```

### 校验规则

- 有效开关 = 全局 && 领域 && 场景 三级合取。
- 全局启用时必须声明 `redis_instance`；只允许全局指定，不支持层级覆盖。
- 声明缺失（domain/scenario 未登记）`Resolve` 返回错误，不静默回退默认 TTL。
- 启用场景 TTL 必须 > 0；禁用场景允许 TTL 为 0，便于再开启时补齐。
- 目录构造后不可变；禁止叠加热更新、动态注册或默认值回退。

## 缓存事件驱动失效

使用 `CacheInvalidator` 接口实现精确缓存失效

## 1. 事件实现接口

```go
// 在领域事件中实现 CacheInvalidator 接口
type ArticleContentUpdatedEvent struct {
    event.BaseEvent
    ArticleID uint
}

// CacheArgs 返回用于构建缓存 key 的参数
func (e *ArticleContentUpdatedEvent) CacheArgs() []any {
    return []any{e.ArticleID}
}
```

## 2. 配置失效规则

```yaml
# config.yaml
cache:
  invalidation_rules:
    - event: article:content:updated
      invalidate:
        - article:markdown:content
```

## 3. Service 发布事件

```go
// 更新内容后发布事件
s.dispatchAsync(ctx, NewArticleContentUpdatedEvent(articleID, "markdown"))
```

**流程**：事件发布 → cache 组件自动订阅 → 通过 `CacheArgs()` 精确失效
