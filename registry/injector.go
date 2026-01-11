// Package registry 提供组件注册中心实现
package registry

import (
	"context"

	"github.com/KOMKZ/go-yogan-framework/component"
	"github.com/KOMKZ/go-yogan-framework/logger"
	"go.uber.org/zap"
)

// ComponentInjector 通用组件注入器
// 简化从 Registry 获取组件并注入到目标的重复代码
type ComponentInjector struct {
	registry *Registry
	logger   *logger.CtxZapLogger
}

// NewInjector 创建组件注入器
func NewInjector(r *Registry, l *logger.CtxZapLogger) *ComponentInjector {
	return &ComponentInjector{registry: r, logger: l}
}

// IsValid 检查注入器是否可用
func (i *ComponentInjector) IsValid() bool {
	return i.registry != nil
}

// Inject 泛型注入方法（包级别函数）
//
// 参数：
//   - i: 注入器实例
//   - ctx: 上下文
//   - name: 组件名称
//   - checker: 可选的检查函数（返回 false 时跳过注入）
//   - injector: 注入操作函数
//
// 返回：
//   - bool: 注入是否成功
//
// 示例：
//
//	injector := registry.NewInjector(c.registry, c.log)
//	registry.Inject(injector, ctx, component.ComponentTelemetry,
//	    func(tc *telemetry.Component) bool { return tc.IsEnabled() },
//	    func(tc *telemetry.Component) { c.tracerProvider = tc.GetTracerProvider() },
//	)
func Inject[T component.Component](
	i *ComponentInjector,
	ctx context.Context,
	name string,
	checker func(T) bool,
	injector func(T),
) bool {
	if i.registry == nil {
		return false
	}

	comp, ok := GetTyped[T](i.registry, name)
	if !ok {
		i.logger.DebugCtx(ctx, "Component not found", zap.String("name", name))
		return false
	}

	// 检查组件是否为 nil（泛型类型可能是指针）
	if any(comp) == nil {
		i.logger.DebugCtx(ctx, "Component is nil", zap.String("name", name))
		return false
	}

	// 执行可选检查
	if checker != nil && !checker(comp) {
		i.logger.DebugCtx(ctx, "Component check failed", zap.String("name", name))
		return false
	}

	// 执行注入
	injector(comp)
	i.logger.DebugCtx(ctx, "✅ Component injected", zap.String("name", name))
	return true
}

// InjectWithResult 泛型注入方法（支持返回结果）
//
// 参数：
//   - i: 注入器实例
//   - ctx: 上下文
//   - name: 组件名称
//   - checker: 可选的检查函数
//   - extractor: 提取函数，从组件中提取需要的值
//
// 返回：
//   - R: 提取的结果
//   - bool: 是否成功
func InjectWithResult[T component.Component, R any](
	i *ComponentInjector,
	ctx context.Context,
	name string,
	checker func(T) bool,
	extractor func(T) R,
) (R, bool) {
	var zero R

	if i.registry == nil {
		return zero, false
	}

	comp, ok := GetTyped[T](i.registry, name)
	if !ok {
		i.logger.DebugCtx(ctx, "Component not found", zap.String("name", name))
		return zero, false
	}

	if any(comp) == nil {
		i.logger.DebugCtx(ctx, "Component is nil", zap.String("name", name))
		return zero, false
	}

	if checker != nil && !checker(comp) {
		i.logger.DebugCtx(ctx, "Component check failed", zap.String("name", name))
		return zero, false
	}

	result := extractor(comp)
	i.logger.DebugCtx(ctx, "✅ Component value extracted", zap.String("name", name))
	return result, true
}

