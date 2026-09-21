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

## 启动期缺失依赖诊断

应用的 `OnSetup` 返回 `samber/do` 缺失服务错误时，`BaseApplication.Setup` 会统一转换为
`di.ResolutionError`：

- ERROR/FATAL 主消息只包含缺失服务、依赖路径和已注册服务数量；
- 完整服务清单只写入 DEBUG 结构化字段 `di_registered_services`；
- `errors.Is(err, do.ErrServiceNotFound)` 与原始错误链保持有效；
- 上游错误格式无法识别时降级为紧凑通用消息，不回退输出全量原文。

应用不得自行截断或解析 DI 错误字符串。非生命周期入口如需相同行为，调用
`di.NormalizeResolutionError(injector, err)`，并从 `*di.ResolutionError` 的访问器读取诊断字段。
