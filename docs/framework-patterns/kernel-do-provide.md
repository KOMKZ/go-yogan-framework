# do.Provide 注册 Provider

## 应用层注册

```go
// 在 callbacks.go 的 initDI() 中
do.Provide(injector, func(i do.Injector) (*domain.Service, error) {
    db := do.MustInvoke[*gorm.DB](i)
    redis, _ := do.Invoke[goredis.UniversalClient](i)
    return domain.NewService(db, redis), nil
})
```

## 领域层依赖注入

```go
// 领域服务通过构造函数接收依赖
type Service struct {
    db          *gorm.DB
    redisClient redis.UniversalClient
}

func NewService(db *gorm.DB, redisClient redis.UniversalClient) *Service {
    return &Service{db: db, redisClient: redisClient}
}
```
