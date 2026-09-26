# Database 内核契约

## 驱动边界

`database.Manager` 默认注册 MySQL 与 PostgreSQL。生产应用只配置实际使用的驱动，不能为了测试便利静态导入 SQLite。

SQLite 是可选驱动。测试工具或确实使用 SQLite 的应用需要显式空白导入：

```go
import _ "github.com/KOMKZ/go-yogan-framework/database/sqlitedriver"
```

该边界保证普通 MySQL/PostgreSQL 消费方的编译依赖图不包含 `go-sqlite3`。新增可选方言时使用 `database.RegisterDialector` 提供独立注册包，不向 `database.Manager` 追加静态 import 或 driver switch。

## 错误翻译

框架创建的 GORM 连接统一启用 `TranslateError`。Repository 判断唯一键冲突时使用：

```go
database.IsDuplicateKeyError(err)
```

该函数识别框架 `ErrDuplicateKey`、GORM `ErrDuplicatedKey`，并兼容直接注入未开启错误翻译的 MySQL 连接所返回的 1062。SQLite 测试 fixture 必须开启 `TranslateError`，禁止在生产代码引用 `mattn/go-sqlite3.Error` 等 CGO-only 类型。

`BaseRepository.Create` 会把唯一键冲突映射为可通过 `errors.Is(err, database.ErrDuplicateKey)` 判断的稳定错误链，同时保留原始 cause。领域 repository 应继续把它映射为本领域契约，service 不识别数据库驱动错误。
