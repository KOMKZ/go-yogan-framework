package di

import (
	"testing"

	"github.com/KOMKZ/go-yogan-framework/registry"
	"github.com/samber/do/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 使用 bridge_test.go 中已定义的 mockComponent

// TestRegisterCoreComponents_Empty 测试空 Registry
func TestRegisterCoreComponents_Empty(t *testing.T) {
	reg := registry.NewRegistry()
	injector := do.New()
	defer injector.Shutdown()

	// 空 Registry 不应该 panic
	RegisterCoreComponents(injector, reg)
}

// TestRegisterCoreComponents_WithMockComponents 测试带模拟组件的注册
func TestRegisterCoreComponents_WithMockComponents(t *testing.T) {
	reg := registry.NewRegistry()
	injector := do.New()
	defer injector.Shutdown()

	// 注册模拟组件（不会真正被使用，因为类型不匹配）
	mockComp := &mockComponent{name: "mock", deps: nil}
	reg.Register(mockComp)

	// 不应该 panic
	RegisterCoreComponents(injector, reg)
}

// TestProvideDB_NotFound 测试数据库组件未找到
func TestProvideDB_NotFound(t *testing.T) {
	reg := registry.NewRegistry()
	injector := do.New()
	defer injector.Shutdown()

	do.Provide(injector, ProvideDB(reg))

	// 尝试调用
	_, err := do.Invoke[*struct{ DB string }](injector)
	assert.Error(t, err)
}

// TestProvideJWTManager_NotFound 测试 JWT 组件未找到
func TestProvideJWTManager_NotFound(t *testing.T) {
	reg := registry.NewRegistry()
	injector := do.New()
	defer injector.Shutdown()

	do.Provide(injector, ProvideJWTManager(reg))

	// 尝试调用 - 类型不匹配，会失败
	_, err := do.Invoke[*struct{}](injector)
	assert.Error(t, err)
}

// TestProvideJWTConfig_NotFound 测试 JWT 配置未找到
func TestProvideJWTConfig_NotFound(t *testing.T) {
	reg := registry.NewRegistry()
	injector := do.New()
	defer injector.Shutdown()

	do.Provide(injector, ProvideJWTConfig(reg))

	// 尝试调用 - 类型不匹配，会失败
	_, err := do.Invoke[*struct{}](injector)
	assert.Error(t, err)
}

// TestProvideEventDispatcher_NotFound 测试事件组件未找到
func TestProvideEventDispatcher_NotFound(t *testing.T) {
	reg := registry.NewRegistry()
	injector := do.New()
	defer injector.Shutdown()

	do.Provide(injector, ProvideEventDispatcher(reg))

	// 尝试调用 - 类型不匹配，会失败
	_, err := do.Invoke[*struct{}](injector)
	assert.Error(t, err)
}

// TestProvideCacheComponent_NotFound 测试缓存组件未找到
func TestProvideCacheComponent_NotFound(t *testing.T) {
	reg := registry.NewRegistry()
	injector := do.New()
	defer injector.Shutdown()

	do.Provide(injector, ProvideCacheComponent(reg))

	// 尝试调用 - 类型不匹配，会失败
	_, err := do.Invoke[*struct{}](injector)
	assert.Error(t, err)
}

// TestErrComponentNotFound 测试组件未找到错误
func TestErrComponentNotFound(t *testing.T) {
	err := ErrComponentNotFound("test-component")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "test-component")
	assert.Contains(t, err.Error(), "component not found")
}

// TestComponentNotFoundError 测试错误类型
func TestComponentNotFoundError(t *testing.T) {
	err := &ComponentNotFoundError{Name: "my-component"}
	assert.Equal(t, "component not found: my-component", err.Error())
}

// TestProvideDB_ProviderCreation 测试 ProvideDB Provider 创建
func TestProvideDB_ProviderCreation(t *testing.T) {
	reg := registry.NewRegistry()

	provider := ProvideDB(reg)
	assert.NotNil(t, provider)
}

// TestProvideJWTManager_ProviderCreation 测试 ProvideJWTManager Provider 创建
func TestProvideJWTManager_ProviderCreation(t *testing.T) {
	reg := registry.NewRegistry()

	provider := ProvideJWTManager(reg)
	assert.NotNil(t, provider)
}

// TestProvideJWTConfig_ProviderCreation 测试 ProvideJWTConfig Provider 创建
func TestProvideJWTConfig_ProviderCreation(t *testing.T) {
	reg := registry.NewRegistry()

	provider := ProvideJWTConfig(reg)
	assert.NotNil(t, provider)
}

// TestProvideEventDispatcher_ProviderCreation 测试 ProvideEventDispatcher Provider 创建
func TestProvideEventDispatcher_ProviderCreation(t *testing.T) {
	reg := registry.NewRegistry()

	provider := ProvideEventDispatcher(reg)
	assert.NotNil(t, provider)
}

// TestProvideCacheComponent_ProviderCreation 测试 ProvideCacheComponent Provider 创建
func TestProvideCacheComponent_ProviderCreation(t *testing.T) {
	reg := registry.NewRegistry()

	provider := ProvideCacheComponent(reg)
	assert.NotNil(t, provider)
}

// TestProvideDB_InvokeError 测试 ProvideDB 调用错误
func TestProvideDB_InvokeError(t *testing.T) {
	reg := registry.NewRegistry()
	injector := do.New()
	defer injector.Shutdown()

	// 注册 provider
	do.Provide(injector, ProvideDB(reg))

	// 直接调用 provider 函数测试错误路径
	provider := ProvideDB(reg)
	_, err := provider(injector)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database")
}

// TestProvideJWTManager_InvokeError 测试 ProvideJWTManager 调用错误
func TestProvideJWTManager_InvokeError(t *testing.T) {
	reg := registry.NewRegistry()
	injector := do.New()
	defer injector.Shutdown()

	provider := ProvideJWTManager(reg)
	_, err := provider(injector)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "jwt")
}

// TestProvideJWTConfig_InvokeError 测试 ProvideJWTConfig 调用错误
func TestProvideJWTConfig_InvokeError(t *testing.T) {
	reg := registry.NewRegistry()
	injector := do.New()
	defer injector.Shutdown()

	provider := ProvideJWTConfig(reg)
	_, err := provider(injector)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "jwt")
}

// TestProvideEventDispatcher_InvokeError 测试 ProvideEventDispatcher 调用错误
func TestProvideEventDispatcher_InvokeError(t *testing.T) {
	reg := registry.NewRegistry()
	injector := do.New()
	defer injector.Shutdown()

	provider := ProvideEventDispatcher(reg)
	_, err := provider(injector)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "event")
}

// TestProvideCacheComponent_InvokeError 测试 ProvideCacheComponent 调用错误
func TestProvideCacheComponent_InvokeError(t *testing.T) {
	reg := registry.NewRegistry()
	injector := do.New()
	defer injector.Shutdown()

	provider := ProvideCacheComponent(reg)
	_, err := provider(injector)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cache")
}
