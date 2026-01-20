// Package errcode provides the basic types and functionalities for hierarchical error codes
package errcode

import (
	"fmt"
	"sync"
)

// Registry for error codes (to prevent conflicts)
type Registry struct {
	mu     sync.RWMutex
	codes  map[int]string // code -> module:msgKey
	locked bool           // Is locked (locked if new error codes are not allowed to be registered)
}

// globalRegistry global error code registry
var globalRegistry = &Registry{
	codes: make(map[int]string),
}

// Register error codes (to prevent conflicts)
// If the error code already exists and the msgKey is different, then panic
func Register(err *LayeredError) *LayeredError {
	return globalRegistry.Register(err)
}

// Register error codes to the registry
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
		// Same error code and key, allow duplicate registration (idempotent)
		return err
	}

	r.codes[code] = key
	return err
}

// Lock registry to prevent new error code registration
// Usually called after application startup to prevent runtime dynamic registration errors for error codes
func (r *Registry) Lock() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.locked = true
}

// Unlock the registry to allow registration of new error codes
func (r *Registry) Unlock() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.locked = false
}

// Check if the registry is locked
func (r *Registry) IsLocked() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.locked
}

// GetAll Retrieve all registered error codes
func (r *Registry) GetAll() map[int]string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	codes := make(map[int]string, len(r.codes))
	for k, v := range r.codes {
		codes[k] = v
	}
	return codes
}

// Count Get the number of registered error codes
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.codes)
}

// Clear registry (for testing only)
func (r *Registry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.codes = make(map[int]string)
	r.locked = false
}

// LockGlobalRegistry Lock the global registry
func LockGlobalRegistry() {
	globalRegistry.Lock()
}

// UnlockGlobalRegistry Unlocks the global registry
func UnlockGlobalRegistry() {
	globalRegistry.Unlock()
}

// Check if the global registry is locked
func IsGlobalRegistryLocked() bool {
	return globalRegistry.IsLocked()
}

// GetAllRegisteredCodes Get all registered error codes
func GetAllRegisteredCodes() map[int]string {
	return globalRegistry.GetAll()
}

// GetRegistryCount Get registered error code count
func GetRegistryCount() int {
	return globalRegistry.Count()
}

// ClearGlobalRegistry Clear global registry (for testing only)
func ClearGlobalRegistry() {
	globalRegistry.Clear()
}

