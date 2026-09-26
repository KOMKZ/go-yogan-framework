// 本文件统一识别数据库唯一键冲突。
// production Manager 会启用 GORM TranslateError，正常入口应得到 gorm.ErrDuplicatedKey；
// MySQL 1062 分支兼容调用方直接注入未开启翻译的 *gorm.DB。SQLite 测试必须开启
// TranslateError，禁止在本包引用 go-sqlite3 的 CGO-only 错误类型。
package database

import (
	"errors"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

// IsDuplicateKeyError 判断错误链是否表示唯一键冲突。
func IsDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrDuplicateKey) || errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}
