package cache

import (
	"net/http"

	"github.com/KOMKZ/go-yogan-framework/errcode"
)

// 模块码
const (
	ModuleCode = 70 // 缓存模块码
)

// 错误码定义
const (
	// 缓存层错误码：70xxxx
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
	// ErrCacheMiss 缓存未命中
	ErrCacheMiss = errcode.New(
		ModuleCode, ErrCodeCacheMiss,
		"cache", "error.cache.miss", "缓存未命中",
		http.StatusOK,
	)

	// ErrStoreNotFound 存储后端未找到
	ErrStoreNotFound = errcode.New(
		ModuleCode, ErrCodeStoreNotFound,
		"cache", "error.cache.store_not_found", "存储后端未找到",
		http.StatusInternalServerError,
	)

	// ErrLoaderNotFound 加载器未找到
	ErrLoaderNotFound = errcode.New(
		ModuleCode, ErrCodeLoaderNotFound,
		"cache", "error.cache.loader_not_found", "缓存加载器未注册",
		http.StatusInternalServerError,
	)

	// ErrSerialize 序列化错误
	ErrSerialize = errcode.New(
		ModuleCode, ErrCodeSerialize,
		"cache", "error.cache.serialize", "序列化失败",
		http.StatusInternalServerError,
	)

	// ErrDeserialize 反序列化错误
	ErrDeserialize = errcode.New(
		ModuleCode, ErrCodeDeserialize,
		"cache", "error.cache.deserialize", "反序列化失败",
		http.StatusInternalServerError,
	)

	// ErrStoreGet 存储获取错误
	ErrStoreGet = errcode.New(
		ModuleCode, ErrCodeStoreGet,
		"cache", "error.cache.store_get", "存储获取失败",
		http.StatusInternalServerError,
	)

	// ErrStoreSet 存储设置错误
	ErrStoreSet = errcode.New(
		ModuleCode, ErrCodeStoreSet,
		"cache", "error.cache.store_set", "存储设置失败",
		http.StatusInternalServerError,
	)

	// ErrStoreDelete 存储删除错误
	ErrStoreDelete = errcode.New(
		ModuleCode, ErrCodeStoreDelete,
		"cache", "error.cache.store_delete", "存储删除失败",
		http.StatusInternalServerError,
	)

	// ErrConfigInvalid 配置无效
	ErrConfigInvalid = errcode.New(
		ModuleCode, ErrCodeConfigInvalid,
		"cache", "error.cache.config_invalid", "缓存配置无效",
		http.StatusInternalServerError,
	)

	// ErrCacheableNotFound 缓存项未找到
	ErrCacheableNotFound = errcode.New(
		ModuleCode, ErrCodeCacheableNotFound,
		"cache", "error.cache.cacheable_not_found", "缓存项未配置",
		http.StatusInternalServerError,
	)
)
