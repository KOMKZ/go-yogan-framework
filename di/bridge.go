// Package di provides dependency injection utilities based on samber/do.
package di

import (
	"context"
	"fmt"

	"github.com/KOMKZ/go-yogan-framework/component"
	"github.com/samber/do/v2"
)

// Bridge 桥接器，连接 Registry 和 samber/do
//
// 设计目的：
//   - 让现有 Registry 中的组件可以被 samber/do 访问
//   - 让 samber/do 中的服务可以被 Registry 中的组件使用
//   - 支持渐进式迁移，两套系统共存
type Bridge struct {
	registry component.Registry
	injector *do.RootScope
}

// NewBridge 创建桥接器
//
// 参数：
//   - registry: 现有的组件注册中心
//   - injector: samber/do 的根注入器
func NewBridge(registry component.Registry, injector *do.RootScope) *Bridge {
	return &Bridge{
		registry: registry,
		injector: injector,
	}
}

// Registry 获取 Registry 实例
func (b *Bridge) Registry() component.Registry {
	return b.registry
}

// Injector 获取 samber/do 注入器
func (b *Bridge) Injector() *do.RootScope {
	return b.injector
}

// ProvideFromRegistry 将 Registry 中的组件暴露给 samber/do
//
// 这个函数从 Registry 获取组件，并注册到 samber/do 容器中。
// 使用时机：初始化阶段，在 Registry.Init() 之后调用。
//
// 示例：
//
//	bridge.ProvideFromRegistry[*database.Component](component.ComponentDatabase)
func ProvideFromRegistry[T component.Component](b *Bridge, name string) error {
	do.Provide(b.injector, func(i do.Injector) (T, error) {
		comp, ok := b.registry.Get(name)
		if !ok {
			var zero T
			return zero, fmt.Errorf("组件 '%s' 未在 Registry 中注册", name)
		}

		typed, ok := comp.(T)
		if !ok {
			var zero T
			return zero, fmt.Errorf("组件 '%s' 类型不匹配，期望 %T", name, zero)
		}

		return typed, nil
	})
	return nil
}

// MustProvideFromRegistry 将 Registry 组件暴露给 do（失败则 panic）
func MustProvideFromRegistry[T component.Component](b *Bridge, name string) {
	if err := ProvideFromRegistry[T](b, name); err != nil {
		panic(err)
	}
}

// ProvideValue 将任意值注册到 samber/do
//
// 适用于将 Registry 中组件的核心对象直接暴露
//
// 示例：
//
//	// 将数据库组件的 *gorm.DB 暴露给 do
//	dbComp := registry.MustGet("database").(*database.Component)
//	bridge.ProvideValue(dbComp.DB())
func ProvideValue[T any](b *Bridge, value T) {
	do.ProvideValue(b.injector, value)
}

// ProvideNamedValue 将命名值注册到 samber/do
func ProvideNamedValue[T any](b *Bridge, name string, value T) {
	do.ProvideNamedValue(b.injector, name, value)
}

// Invoke 从 samber/do 获取服务
func Invoke[T any](b *Bridge) (T, error) {
	return do.Invoke[T](b.injector)
}

// MustInvoke 从 samber/do 获取服务（失败则 panic）
func MustInvoke[T any](b *Bridge) T {
	return do.MustInvoke[T](b.injector)
}

// InvokeNamed 从 samber/do 获取命名服务
func InvokeNamed[T any](b *Bridge, name string) (T, error) {
	return do.InvokeNamed[T](b.injector, name)
}

// MustInvokeNamed 从 samber/do 获取命名服务（失败则 panic）
func MustInvokeNamed[T any](b *Bridge, name string) T {
	return do.MustInvokeNamed[T](b.injector, name)
}

// Provide 注册服务提供者到 samber/do
func Provide[T any](b *Bridge, provider func(do.Injector) (T, error)) {
	do.Provide(b.injector, provider)
}

// ProvideNamed 注册命名服务提供者到 samber/do
func ProvideNamed[T any](b *Bridge, name string, provider func(do.Injector) (T, error)) {
	do.ProvideNamed(b.injector, name, provider)
}

// Shutdown 优雅关闭 samber/do 容器
//
// 返回关闭过程中的错误（如果有）
func (b *Bridge) Shutdown() error {
	return b.injector.Shutdown()
}

// ShutdownWithContext 带上下文的优雅关闭
func (b *Bridge) ShutdownWithContext(ctx context.Context) error {
	done := make(chan error, 1)
	go func() {
		done <- b.Shutdown()
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// HealthCheck 执行 samber/do 容器的健康检查
//
// 返回 map[服务名]error，如果所有服务健康则 map 为空
func (b *Bridge) HealthCheck() map[string]error {
	return b.injector.HealthCheck()
}

// HealthCheckWithContext 带上下文的健康检查
func (b *Bridge) HealthCheckWithContext(ctx context.Context) map[string]error {
	return b.injector.HealthCheckWithContext(ctx)
}

// IsHealthy 检查是否所有服务都健康
func (b *Bridge) IsHealthy() bool {
	errors := b.HealthCheck()
	return len(errors) == 0
}

// IsHealthyWithContext 带上下文检查是否所有服务都健康
func (b *Bridge) IsHealthyWithContext(ctx context.Context) bool {
	errors := b.HealthCheckWithContext(ctx)
	return len(errors) == 0
}
