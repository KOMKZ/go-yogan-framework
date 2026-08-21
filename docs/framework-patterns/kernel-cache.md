# 缓存事件驱动失效

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
