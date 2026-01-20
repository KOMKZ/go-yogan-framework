package cache

import (
	"net/http"

	"github.com/KOMKZ/go-yogan-framework/errcode"
)

// module code
const (
	ModuleCode = 70 // Cache module code
)

// Error code definitions
const (
	// Cache layer error code: 70xxxx
	ErrCodeCacheMiss         = 1
	ErrCodeStoreNotFound     = 2
	ErrCodeLoaderNotFound    = 3
	ErrCodeSerialize         = 4
	ErrCodeDeserialize       = 5
	ErrCodeStoreGet          = 6
	ErrCodeStoreSet          = 7
	ErrCodeStoreDelete       = 8
	ErrCodeConfigInvalid     = 9
	ErrCodeCacheableNotFound = 10
)

var (
	// ErrCacheMiss cache miss
	ErrCacheMiss = errcode.New(
		ModuleCode, ErrCodeCacheMiss,
		"cache", "error.cache.miss", "缓存未命中",
		http.StatusOK,
	)

	// ErrStoreNotFound Storage backend not found
	ErrStoreNotFound = errcode.New(
		ModuleCode, ErrCodeStoreNotFound,
		"cache", "error.cache.store_not_found", "存储后端未找到",
		http.StatusInternalServerError,
	)

	// ErrorLoaderNotFound Loader not found
	ErrLoaderNotFound = errcode.New(
		ModuleCode, ErrCodeLoaderNotFound,
		"cache", "error.cache.loader_not_found", "缓存加载器未注册",
		http.StatusInternalServerError,
	)

	// ErrSerialize serialization error
	ErrSerialize = errcode.New(
		ModuleCode, ErrCodeSerialize,
		"cache", "error.cache.serialize", "序列化失败",
		http.StatusInternalServerError,
	)

	// Deserialization error
	ErrDeserialize = errcode.New(
		ModuleCode, ErrCodeDeserialize,
		"cache", "error.cache.deserialize", "反序列化失败",
		http.StatusInternalServerError,
	)

	// ErrStoreGet Store retrieval error
	ErrStoreGet = errcode.New(
		ModuleCode, ErrCodeStoreGet,
		"cache", "error.cache.store_get", "存储获取失败",
		http.StatusInternalServerError,
	)

	// ErrorStoreSettings store settings error
	ErrStoreSet = errcode.New(
		ModuleCode, ErrCodeStoreSet,
		"cache", "error.cache.store_set", "存储设置失败",
		http.StatusInternalServerError,
	)

	// ErrStoreDelete storage deletion error
	ErrStoreDelete = errcode.New(
		ModuleCode, ErrCodeStoreDelete,
		"cache", "error.cache.store_delete", "存储删除失败",
		http.StatusInternalServerError,
	)

	// ErrConfigInvalid Configuration invalid
	ErrConfigInvalid = errcode.New(
		ModuleCode, ErrCodeConfigInvalid,
		"cache", "error.cache.config_invalid", "缓存配置无效",
		http.StatusInternalServerError,
	)

	// ErrCacheableNotFound Cache item not found
	ErrCacheableNotFound = errcode.New(
		ModuleCode, ErrCodeCacheableNotFound,
		"cache", "error.cache.cacheable_not_found", "缓存项未配置",
		http.StatusInternalServerError,
	)
)
