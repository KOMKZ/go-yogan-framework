// 本文件声明数据库公共 sentinel；驱动错误分类实现见 duplicate_key_error.go。
// 消费方应通过 errors.Is 判断这些稳定契约，不应识别驱动错误文案。
package database

import "errors"

var (
	// ErrInvalidConfig Invalid Configuration
	ErrInvalidConfig = errors.New("invalid database config")

	// ErrRecordNotFound Record not found
	ErrRecordNotFound = errors.New("record not found")

	// ErrDuplicateKey Primary key or unique key conflict
	ErrDuplicateKey = errors.New("duplicate key")

	// ErrConnectionFailed Connection failed
	ErrConnectionFailed = errors.New("database connection failed")
)
