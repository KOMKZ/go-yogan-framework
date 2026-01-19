// Package registry 提供组件注册中心实现
// 作为独立内核组件，不依赖任何业务组件
//
// Deprecated: 此包已废弃，请使用 github.com/KOMKZ/go-yogan-framework/di 包
// 新代码应使用 samber/do 进行依赖注入
// 迁移指南：参考 di.DoApplication 和 di.Provider* 系列函数
package registry

import (
	"context"
	"fmt"
	"sync"

	"github.com/KOMKZ/go-yogan-framework/component"
	"github.com/KOMKZ/go-yogan-framework/logger"
	"go.uber.org/zap"
)

// Registry 组件注册中心实现
// 实现 component.Registry 接口
//
// Deprecated: 请使用 samber/do 进行依赖注入
// 迁移方法：使用 di.NewDoApplication() 替代
type Registry struct {
	mu         sync.RWMutex
	components map[string]component.Component
	logger     *logger.CtxZapLogger // 可选的日志组件（后注入）
}

// NewRegistry 创建组件注册中心
//
// Deprecated: 请使用 do.New() 创建 samber/do 注入器
func NewRegistry() *Registry {
	return &Registry{
		components: make(map[string]component.Component),
	}
}

// Register 注册组件
func (r *Registry) Register(comp component.Component) error {
	if comp == nil {
		return fmt.Errorf("组件不能为空")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	name := comp.Name()
	if name == "" {
		return fmt.Errorf("组件名称不能为空")
	}

	if _, exists := r.components[name]; exists {
		return fmt.Errorf("组件 '%s' 已存在", name)
	}

	r.components[name] = comp

	// 🎯 如果组件有 SetRegistry 方法，自动注入注册中心引用（具体类型）
	if setter, ok := comp.(interface{ SetRegistry(*Registry) }); ok {
		setter.SetRegistry(r)
	}

	return nil
}

// MustRegister 注册组件（失败则 panic）
// 适用于核心组件注册，失败时采用 Fail Fast 策略
func (r *Registry) MustRegister(comp component.Component) {
	if err := r.Register(comp); err != nil {
		panic(fmt.Sprintf("注册核心组件 '%s' 失败: %v", comp.Name(), err))
	}
}

// SetLogger 设置日志组件（安全方法：只允许设置一次）
//
// 设计原则：
//   - 在 NewBase 中调用，Registry 创建后立即注入
//   - 只允许设置一次，重复设置会 panic（防止误用）
//   - Init/Start/Stop 全流程都有日志能力
func (r *Registry) SetLogger(l *logger.CtxZapLogger) {
	if r.logger != nil {
		panic("Registry logger 已设置，禁止重复设置")
	}
	if l == nil {
		panic("Registry logger 不能为 nil")
	}
	r.logger = l
}

// logInfo 安全的日志记录（Logger 未初始化时静默忽略）
func (r *Registry) logInfo(ctx context.Context, msg string, fields ...zap.Field) {
	if r.logger != nil {
		r.logger.InfoCtx(ctx, msg, fields...)
	}
}

// logDebug 安全的 Debug 日志
func (r *Registry) logDebug(ctx context.Context, msg string, fields ...zap.Field) {
	if r.logger != nil {
		r.logger.DebugCtx(ctx, msg, fields...)
	}
}

// logWarn 安全的 Warn 日志
func (r *Registry) logWarn(ctx context.Context, msg string, fields ...zap.Field) {
	if r.logger != nil {
		r.logger.WarnCtx(ctx, msg, fields...)
	}
}

// logError 安全的错误日志
func (r *Registry) logError(ctx context.Context, msg string, fields ...zap.Field) {
	if r.logger != nil {
		r.logger.ErrorCtx(ctx, msg, fields...)
	}
}

// Get 获取组件
func (r *Registry) Get(name string) (component.Component, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	comp, ok := r.components[name]
	return comp, ok
}

// MustGet 获取组件（不存在则 panic）
func (r *Registry) MustGet(name string) component.Component {
	comp, ok := r.Get(name)
	if !ok {
		panic(fmt.Sprintf("组件 '%s' 不存在", name))
	}
	return comp
}

// GetTyped 泛型函数获取组件并自动类型转换（包级别函数）
//
// Deprecated: 请使用 samber/do.Invoke 获取组件
// 迁移示例：
//
//	// 旧代码：redisComp, ok := registry.GetTyped[*redis.Component](reg, "redis")
//	// 新代码：redisComp, err := do.Invoke[*redis.Component](injector)
//
// 参数：
//   - r: Registry 实例
//   - name: 组件名称
//
// 返回：
//   - T: 组件实例（已转换为目标类型）
//   - bool: 组件是否存在且类型匹配
func GetTyped[T component.Component](r *Registry, name string) (T, bool) {
	var zero T
	comp, ok := r.Get(name)
	if !ok {
		return zero, false
	}

	typed, ok := comp.(T)
	if !ok {
		return zero, false
	}

	return typed, true
}

// MustGetTyped 泛型函数获取组件（不存在或类型不匹配则 panic）（包级别函数）
//
// Deprecated: 请使用 samber/do.MustInvoke 获取组件
// 迁移示例：
//
//	// 旧代码：redisComp := registry.MustGetTyped[*redis.Component](reg, "redis")
//	// 新代码：redisComp := do.MustInvoke[*redis.Component](injector)
//
// 参数：
//   - r: Registry 实例
//   - name: 组件名称
//
// 返回：
//   - T: 组件实例（已转换为目标类型）
func MustGetTyped[T component.Component](r *Registry, name string) T {
	typed, ok := GetTyped[T](r, name)
	if !ok {
		var zero T
		panic(fmt.Sprintf("组件 '%s' 不存在或类型不匹配，期望类型: %T", name, zero))
	}
	return typed
}

// Has 检查组件是否存在
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.components[name]
	return exists
}

// Resolve 拓扑排序，返回按依赖顺序排列的组件
func (r *Registry) Resolve() ([]component.Component, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	order, err := r.topologicalSort()
	if err != nil {
		return nil, err
	}
	return order, nil
}

// Init 初始化所有组件
//
// 🎯 重构后：传递 ConfigLoader 给组件，而不是 Registry
func (r *Registry) Init(ctx context.Context) error {
	r.logInfo(ctx, "🚀 开始初始化组件", zap.Int("total", len(r.components)))

	// 首先获取 ConfigComponent（它实现了 ConfigLoader 接口）
	configComp, ok := r.Get(component.ComponentConfig)
	if !ok {
		return fmt.Errorf("配置组件未注册")
	}

	// ConfigComponent 实现了 component.ConfigLoader 接口
	loader, ok := configComp.(component.ConfigLoader)
	if !ok {
		return fmt.Errorf("配置组件未实现 ConfigLoader 接口")
	}

	// 解析依赖层级
	layers, err := r.resolveLayers()
	if err != nil {
		r.logError(ctx, "❌ 解析组件依赖失败", zap.Error(err))
		return fmt.Errorf("解析组件依赖失败: %w", err)
	}

	r.logDebug(ctx, "组件依赖层级解析完成", zap.Int("layers", len(layers)))

	// 按层级初始化组件，传递 ConfigLoader
	for layerIdx, layer := range layers {
		r.logDebug(ctx, "初始化组件层",
			zap.Int("layer", layerIdx),
			zap.Int("count", len(layer)))

		if err := r.runLayer(ctx, layer, func(c component.Component) error {
			r.logDebug(ctx, "初始化组件", zap.String("component", c.Name()))
			return c.Init(ctx, loader) // ← 传递 ConfigLoader
		}); err != nil {
			r.logError(ctx, "❌ 组件初始化失败", zap.Error(err))
			return err
		}
	}

	r.logInfo(ctx, "✅ 所有组件初始化完成")
	return nil
}

// Start 启动所有组件
func (r *Registry) Start(ctx context.Context) error {
	r.logInfo(ctx, "🚀 开始启动组件")

	layers, err := r.resolveLayers()
	if err != nil {
		r.logError(ctx, "❌ 解析组件依赖失败", zap.Error(err))
		return fmt.Errorf("解析组件依赖失败: %w", err)
	}

	for layerIdx, layer := range layers {
		r.logDebug(ctx, "启动组件层",
			zap.Int("layer", layerIdx),
			zap.Int("count", len(layer)))

		if err := r.runLayer(ctx, layer, func(c component.Component) error {
			r.logDebug(ctx, "启动组件", zap.String("component", c.Name()))
			return c.Start(ctx)
		}); err != nil {
			r.logError(ctx, "❌ 组件启动失败", zap.Error(err))
			return err
		}
	}

	r.logInfo(ctx, "✅ 所有组件启动完成")
	return nil
}

// Stop 停止所有组件（反向顺序）
func (r *Registry) Stop(ctx context.Context) error {
	r.logInfo(ctx, "🛑 开始停止组件")

	layers, err := r.resolveLayers()
	if err != nil {
		r.logError(ctx, "❌ 解析组件依赖失败", zap.Error(err))
		return fmt.Errorf("解析组件依赖失败: %w", err)
	}

	// 反向停止组件
	for i := len(layers) - 1; i >= 0; i-- {
		r.logDebug(ctx, "停止组件层",
			zap.Int("layer", i),
			zap.Int("count", len(layers[i])))

		r.stopLayer(ctx, layers[i])
	}

	r.logInfo(ctx, "✅ 所有组件已停止")
	return nil
}

// runLayer 并发执行单层组件的某个生命周期函数
func (r *Registry) runLayer(ctx context.Context, layer []component.Component, fn func(component.Component) error) error {
	if len(layer) == 0 {
		return nil
	}

	if len(layer) == 1 {
		comp := layer[0]
		if err := fn(comp); err != nil {
			return fmt.Errorf("组件 '%s' 执行失败: %w", comp.Name(), err)
		}
		return nil
	}

	type result struct {
		comp component.Component
		err  error
	}

	results := make(chan result, len(layer))

	for _, comp := range layer {
		go func(c component.Component) {
			results <- result{
				comp: c,
				err:  fn(c),
			}
		}(comp)
	}

	for range layer {
		res := <-results
		if res.err != nil {
			return fmt.Errorf("组件 '%s' 执行失败: %w", res.comp.Name(), res.err)
		}
	}

	return nil
}

// stopLayer 并发停止单层组件（忽略错误）
func (r *Registry) stopLayer(ctx context.Context, layer []component.Component) {
	if len(layer) == 0 {
		return
	}

	var wg sync.WaitGroup
	for _, comp := range layer {
		wg.Add(1)
		go func(c component.Component) {
			defer wg.Done()
			_ = c.Stop(ctx)
		}(comp)
	}

	wg.Wait()
}

// resolveLayers 将拓扑排序结果按层分组，方便并发执行
func (r *Registry) resolveLayers() ([][]component.Component, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 构建依赖图
	inDegree := make(map[string]int)
	graph := make(map[string][]string)

	for name := range r.components {
		inDegree[name] = 0
		graph[name] = []string{}
	}

	for name, comp := range r.components {
		for _, dep := range comp.DependsOn() {
			// 🎯 支持可选依赖：以 "optional:" 前缀标记
			// 示例：[]string{"config", "logger", "optional:telemetry"}
			depName := dep
			isOptional := false
			if len(dep) > 9 && dep[:9] == "optional:" {
				depName = dep[9:]
				isOptional = true
			}

			// 检查依赖是否存在
			if _, ok := r.components[depName]; !ok {
				if !isOptional {
					// 强制依赖：未找到则报错
					return nil, fmt.Errorf("组件 '%s' 依赖 '%s' 未注册", name, depName)
				}
				// 可选依赖：未找到则跳过
				continue
			}

			// 依赖存在：添加到依赖图
			graph[depName] = append(graph[depName], name)
			inDegree[name]++
		}
	}

	var layers [][]component.Component
	processed := make(map[string]bool)

	for len(processed) < len(r.components) {
		var currentLayer []string
		for name, degree := range inDegree {
			if processed[name] {
				continue
			}
			if degree == 0 {
				currentLayer = append(currentLayer, name)
				processed[name] = true
			}
		}

		if len(currentLayer) == 0 {
			return nil, fmt.Errorf("检测到循环依赖")
		}

		layer := make([]component.Component, 0, len(currentLayer))
		for _, name := range currentLayer {
			layer = append(layer, r.components[name])
			for _, next := range graph[name] {
				inDegree[next]--
			}
		}

		layers = append(layers, layer)
	}

	return layers, nil
}

// topologicalSort 返回拓扑排序结果
func (r *Registry) topologicalSort() ([]component.Component, error) {
	layers, err := r.resolveLayers()
	if err != nil {
		return nil, err
	}

	var result []component.Component
	for _, layer := range layers {
		result = append(result, layer...)
	}
	return result, nil
}
