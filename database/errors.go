package database

import "errors"

var (
	// ErrInvalidConfig 无效配置
	ErrInvalidConfig = errors.New("invalid database config")

	// ErrRecordNotFound 记录不存在
	ErrRecordNotFound = errors.New("record not found")

	// ErrDuplicateKey 主键或唯一键冲突
	ErrDuplicateKey = errors.New("duplicate key")

	// ErrConnectionFailed 连接失败
	ErrConnectionFailed = errors.New("database connection failed")
)

