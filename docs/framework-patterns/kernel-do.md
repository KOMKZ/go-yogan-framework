# 直接使用 samber/do

```go
import "github.com/samber/do/v2"

// 获取 injector
injector := app.GetInjector()

// 获取组件（类型安全）
db := do.MustInvoke[*gorm.DB](injector)
redis := do.MustInvoke[goredis.UniversalClient](injector)
jwtMgr := do.MustInvoke[jwt.TokenManager](injector)

// 可选获取（不存在时返回 error）
cacheOrch, err := do.Invoke[*cache.DefaultOrchestrator](injector)
if err == nil && cacheOrch != nil {
    // 使用缓存
}

// 注册应用层 Provider
do.Provide(injector, func(i do.Injector) (*MyService, error) {
    db := do.MustInvoke[*gorm.DB](i)
    return NewMyService(db), nil
})
```

## 领域层使用

```go
// 领域服务直接依赖内核类型
type Service struct {
    db          *gorm.DB
    redisClient redis.UniversalClient
}

func NewService(db *gorm.DB, redisClient redis.UniversalClient) *Service {
    return &Service{db: db, redisClient: redisClient}
}
```

## 应用层 Provider 注入

```go
// 在 callbacks.go 的 initDI() 中
do.Provide(injector, func(i do.Injector) (*domain.Service, error) {
    db := do.MustInvoke[*gorm.DB](i)
    redis, _ := do.Invoke[goredis.UniversalClient](i)
    return domain.NewService(db, redis), nil
})
```
