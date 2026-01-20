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

