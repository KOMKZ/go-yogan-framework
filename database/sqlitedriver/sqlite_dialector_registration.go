// 本文件把 SQLite 方言作为可选能力注册到 framework database.Manager。
// 测试工具或确实使用 SQLite 的应用通过空白导入本包启用；MySQL/PostgreSQL 生产应用
// 不得为测试便利导入本包，以免把 go-sqlite3 的 CGO 依赖带入发布编译链。
package sqlitedriver

import (
	"github.com/KOMKZ/go-yogan-framework/database"
	"gorm.io/driver/sqlite"
)

func init() {
	if err := database.RegisterDialector("sqlite", sqlite.Open); err != nil {
		panic(err)
	}
}
