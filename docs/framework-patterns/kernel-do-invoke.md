# do.Invoke 获取组件

```go
import "github.com/samber/do/v2"

// 获取 injector
injector := app.GetInjector()

// 必须存在（不存在则 panic）
db := do.MustInvoke[*gorm.DB](injector)
redis := do.MustInvoke[goredis.UniversalClient](injector)
jwtMgr := do.MustInvoke[jwt.TokenManager](injector)

// 可选获取（不存在时返回 error）
cacheOrch, err := do.Invoke[*cache.DefaultOrchestrator](injector)
if err == nil && cacheOrch != nil {
    // 使用缓存
}
```
