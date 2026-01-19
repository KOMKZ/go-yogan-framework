package di

import (
	"context"
	"testing"

	"github.com/KOMKZ/go-yogan-framework/component"
	"github.com/samber/do/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockComponent 用于测试的模拟组件
type mockComponent struct {
	name      string
	deps      []string
	initErr   error
	startErr  error
	stopErr   error
	initCount int
	startCount int
	stopCount  int
}

func (m *mockComponent) Name() string           { return m.name }
func (m *mockComponent) DependsOn() []string    { return m.deps }
func (m *mockComponent) Init(ctx context.Context, loader component.ConfigLoader) error {
	m.initCount++
	return m.initErr
}
func (m *mockComponent) Start(ctx context.Context) error {
	m.startCount++
	return m.startErr
}
func (m *mockComponent) Stop(ctx context.Context) error {
	m.stopCount++
	return m.stopErr
}

// mockRegistry 用于测试的模拟注册中心
type mockRegistry struct {
	components map[string]component.Component
}

func newMockRegistry() *mockRegistry {
	return &mockRegistry{
		components: make(map[string]component.Component),
	}
}

func (r *mockRegistry) Register(comp component.Component) error {
	r.components[comp.Name()] = comp
	return nil
}

func (r *mockRegistry) Get(name string) (component.Component, bool) {
	comp, ok := r.components[name]
	return comp, ok
}

func (r *mockRegistry) MustGet(name string) component.Component {
	comp, ok := r.Get(name)
	if !ok {
		panic("component not found: " + name)
	}
	return comp
}

func (r *mockRegistry) Has(name string) bool {
	_, ok := r.components[name]
	return ok
}

func (r *mockRegistry) Resolve() ([]component.Component, error) {
	var result []component.Component
	for _, comp := range r.components {
		result = append(result, comp)
	}
	return result, nil
}

func (r *mockRegistry) Init(ctx context.Context) error   { return nil }
func (r *mockRegistry) Start(ctx context.Context) error  { return nil }
func (r *mockRegistry) Stop(ctx context.Context) error   { return nil }

func TestNewBridge(t *testing.T) {
	registry := newMockRegistry()
	injector := do.New()
	defer injector.Shutdown()

	bridge := NewBridge(registry, injector)

	assert.NotNil(t, bridge)
	assert.Equal(t, registry, bridge.Registry())
	assert.Equal(t, injector, bridge.Injector())
}

func TestBridge_ProvideFromRegistry(t *testing.T) {
	registry := newMockRegistry()
	comp := &mockComponent{name: "test-component"}
	registry.Register(comp)

	injector := do.New()
	defer injector.Shutdown()

	bridge := NewBridge(registry, injector)

	// 将 Registry 中的组件暴露给 do
	err := ProvideFromRegistry[*mockComponent](bridge, "test-component")
	require.NoError(t, err)

	// 从 do 获取组件
	retrieved, err := Invoke[*mockComponent](bridge)
	require.NoError(t, err)
	assert.Equal(t, comp, retrieved)
}

func TestBridge_ProvideFromRegistry_NotFound(t *testing.T) {
	registry := newMockRegistry()
	injector := do.New()
	defer injector.Shutdown()

	bridge := NewBridge(registry, injector)

	// 尝试暴露不存在的组件
	err := ProvideFromRegistry[*mockComponent](bridge, "not-exist")
	require.NoError(t, err) // Provide 不会立即报错

	// 调用时才报错
	_, err = Invoke[*mockComponent](bridge)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not-exist")
}

func TestBridge_ProvideValue(t *testing.T) {
	registry := newMockRegistry()
	injector := do.New()
	defer injector.Shutdown()

	bridge := NewBridge(registry, injector)

	type MyService struct {
		Name string
	}

	svc := &MyService{Name: "test"}
	ProvideValue(bridge, svc)

	retrieved := MustInvoke[*MyService](bridge)
	assert.Equal(t, svc, retrieved)
}

func TestBridge_ProvideNamedValue(t *testing.T) {
	registry := newMockRegistry()
	injector := do.New()
	defer injector.Shutdown()

	bridge := NewBridge(registry, injector)

	type Config struct {
		Value string
	}

	config1 := &Config{Value: "config1"}
	config2 := &Config{Value: "config2"}

	ProvideNamedValue(bridge, "config1", config1)
	ProvideNamedValue(bridge, "config2", config2)

	retrieved1 := MustInvokeNamed[*Config](bridge, "config1")
	retrieved2 := MustInvokeNamed[*Config](bridge, "config2")

	assert.Equal(t, config1, retrieved1)
	assert.Equal(t, config2, retrieved2)
}

func TestBridge_Provide(t *testing.T) {
	registry := newMockRegistry()
	injector := do.New()
	defer injector.Shutdown()

	bridge := NewBridge(registry, injector)

	type Database struct {
		DSN string
	}

	type Service struct {
		DB *Database
	}

	// 注册 Database
	Provide(bridge, func(i do.Injector) (*Database, error) {
		return &Database{DSN: "test-dsn"}, nil
	})

	// 注册依赖 Database 的 Service
	Provide(bridge, func(i do.Injector) (*Service, error) {
		db := do.MustInvoke[*Database](i)
		return &Service{DB: db}, nil
	})

	// 验证依赖注入
	svc := MustInvoke[*Service](bridge)
	assert.NotNil(t, svc)
	assert.Equal(t, "test-dsn", svc.DB.DSN)
}

func TestBridge_Shutdown(t *testing.T) {
	registry := newMockRegistry()
	injector := do.New()

	bridge := NewBridge(registry, injector)

	type CleanupService struct{}

	do.ProvideNamed(injector, "cleanup", func(i do.Injector) (*CleanupService, error) {
		return &CleanupService{}, nil
	})

	// 触发服务创建
	do.MustInvokeNamed[*CleanupService](injector, "cleanup")

	// 关闭 - 可能返回 error 但不应该 panic
	_ = bridge.Shutdown()
}

func TestBridge_ShutdownWithContext(t *testing.T) {
	registry := newMockRegistry()
	injector := do.New()

	bridge := NewBridge(registry, injector)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 关闭 - 可能返回 error 但不应该 panic
	_ = bridge.ShutdownWithContext(ctx)
}

func TestBridge_HealthCheck(t *testing.T) {
	registry := newMockRegistry()
	injector := do.New()
	defer injector.Shutdown()

	bridge := NewBridge(registry, injector)

	// 没有服务时应该健康
	assert.True(t, bridge.IsHealthy())

	errors := bridge.HealthCheck()
	assert.Empty(t, errors)
}

func TestBridge_HealthCheckWithContext(t *testing.T) {
	registry := newMockRegistry()
	injector := do.New()
	defer injector.Shutdown()

	bridge := NewBridge(registry, injector)

	ctx := context.Background()

	// 没有服务时应该健康
	assert.True(t, bridge.IsHealthyWithContext(ctx))

	errors := bridge.HealthCheckWithContext(ctx)
	assert.Empty(t, errors)
}

func TestBridge_MustProvideFromRegistry(t *testing.T) {
	registry := newMockRegistry()
	comp := &mockComponent{name: "must-test-component"}
	registry.Register(comp)

	injector := do.New()
	defer injector.Shutdown()

	bridge := NewBridge(registry, injector)

	// 不应该 panic
	MustProvideFromRegistry[*mockComponent](bridge, "must-test-component")

	// 从 do 获取组件
	retrieved := MustInvoke[*mockComponent](bridge)
	assert.Equal(t, comp, retrieved)
}

func TestBridge_InvokeNamed(t *testing.T) {
	registry := newMockRegistry()
	injector := do.New()
	defer injector.Shutdown()

	bridge := NewBridge(registry, injector)

	type NamedService struct {
		ID string
	}

	svc := &NamedService{ID: "named-svc"}
	ProvideNamedValue(bridge, "my-service", svc)

	// 使用 InvokeNamed 获取
	retrieved, err := InvokeNamed[*NamedService](bridge, "my-service")
	require.NoError(t, err)
	assert.Equal(t, svc, retrieved)
}

func TestBridge_ProvideNamed(t *testing.T) {
	registry := newMockRegistry()
	injector := do.New()
	defer injector.Shutdown()

	bridge := NewBridge(registry, injector)

	type Counter struct {
		Value int
	}

	// 使用 ProvideNamed 注册
	ProvideNamed(bridge, "counter-1", func(i do.Injector) (*Counter, error) {
		return &Counter{Value: 1}, nil
	})

	ProvideNamed(bridge, "counter-2", func(i do.Injector) (*Counter, error) {
		return &Counter{Value: 2}, nil
	})

	// 获取不同的命名服务
	c1 := MustInvokeNamed[*Counter](bridge, "counter-1")
	c2 := MustInvokeNamed[*Counter](bridge, "counter-2")

	assert.Equal(t, 1, c1.Value)
	assert.Equal(t, 2, c2.Value)
}

func TestBridge_ShutdownWithContext_Timeout(t *testing.T) {
	registry := newMockRegistry()
	injector := do.New()

	bridge := NewBridge(registry, injector)

	// 使用已取消的 context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	// ShutdownWithContext 应该处理取消
	_ = bridge.ShutdownWithContext(ctx)
}
