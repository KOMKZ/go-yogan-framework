# apputil 快捷访问

```go
import "github.com/KOMKZ/go-yogan-apps/pkg/apputil"

// 数据库
db := apputil.MustDB(app)                    // 必须存在，否则 panic
db := apputil.GetDB(app)                     // 可选，返回 nil 表示不可用
mgr := apputil.MustDBManager(app)            // 获取 Manager（多实例）

// Redis
client := apputil.MustRedisClient(app)       // 默认 main 实例
client := apputil.GetRedisClient(app)        // 可选
mgr := apputil.MustRedisManager(app)         // 获取 Manager（多实例）

// JWT
tm := apputil.MustJWTManager(app)            // TokenManager
cfg := apputil.MustJWTConfig(app)            // JWT 配置

// 事件
dispatcher := apputil.MustEventDispatcher(app)
dispatcher := apputil.GetEventDispatcher(app)  // 可选

// 缓存编排
cacheOrch := apputil.MustCacheOrchestrator(app)
cacheOrch := apputil.GetCacheOrchestrator(app)  // 可选

// 邮件组件
emailComp := apputil.MustEmailComponent(app)
emailComp := apputil.GetEmailComponent(app)  // 可选

// 认证
authSvc := apputil.MustAuthService(app)

// Kafka
kafkaMgr := apputil.GetKafkaManager(app)

// 健康检查
healthAgg := apputil.GetHealthAggregator(app)

// 限流
limiterMgr := apputil.GetLimiterManager(app)
```
