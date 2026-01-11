// Package errcode 提供分层错误码的基础类型和功能
package errcode

import (
	"fmt"
	"sync"
)

// Registry 错误码注册表（防止错误码冲突）
type Registry struct {
	mu     sync.RWMutex
	codes  map[int]string // code -> module:msgKey
	locked bool           // 是否锁定（锁定后不允许注册新错误码）
}

// globalRegistry 全局错误码注册表
var globalRegistry = &Registry{
	codes: make(map[int]string),
}

// Register 注册错误码（防止冲突）
// 如果错误码已存在且 msgKey 不同，则 panic
func Register(err *LayeredError) *LayeredError {
	return globalRegistry.Register(err)
}

// Register 注册错误码到注册表
func (r *Registry) Register(err *LayeredError) *LayeredError {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.locked {
		panic(fmt.Sprintf("registry is locked, cannot register error code: %d", err.Code()))
	}

	code := err.Code()
	key := fmt.Sprintf("%s:%s", err.Module(), err.MsgKey())

	if existingKey, exists := r.codes[code]; exists {
		if existingKey != key {
			panic(fmt.Sprintf(
				"error code conflict: code %d is already registered as %s, cannot register as %s",
				code, existingKey, key,
			))
		}
		// 相同错误码和键，允许重复注册（幂等）
		return err
	}

	r.codes[code] = key
	return err
}

// Lock 锁定注册表，阻止新错误码注册
// 通常在应用启动完成后调用，防止运行时动态注册错误码
func (r *Registry) Lock() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.locked = true
}

// Unlock 解锁注册表，允许注册新错误码
func (r *Registry) Unlock() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.locked = false
}

// IsLocked 检查注册表是否已锁定
func (r *Registry) IsLocked() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.locked
}

// GetAll 获取所有已注册的错误码
func (r *Registry) GetAll() map[int]string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	codes := make(map[int]string, len(r.codes))
	for k, v := range r.codes {
		codes[k] = v
	}
	return codes
}

// Count 获取已注册错误码数量
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.codes)
}

// Clear 清空注册表（仅用于测试）
func (r *Registry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.codes = make(map[int]string)
	r.locked = false
}

// LockGlobalRegistry 锁定全局注册表
func LockGlobalRegistry() {
	globalRegistry.Lock()
}

// UnlockGlobalRegistry 解锁全局注册表
func UnlockGlobalRegistry() {
	globalRegistry.Unlock()
}

// IsGlobalRegistryLocked 检查全局注册表是否已锁定
func IsGlobalRegistryLocked() bool {
	return globalRegistry.IsLocked()
}

// GetAllRegisteredCodes 获取所有已注册的错误码
func GetAllRegisteredCodes() map[int]string {
	return globalRegistry.GetAll()
}

// GetRegistryCount 获取已注册错误码数量
func GetRegistryCount() int {
	return globalRegistry.Count()
}

// ClearGlobalRegistry 清空全局注册表（仅用于测试）
func ClearGlobalRegistry() {
	globalRegistry.Clear()
}

