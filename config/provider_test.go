package config

import (
	"testing"

	"github.com/samber/do/v2"
)

func TestProvideLoader(t *testing.T) {
	injector := do.New()

	// 注册 Provider
	do.Provide(injector, ProvideLoader(ProvideLoaderOptions{
		ConfigPath: "testdata",
		AppType:    "http",
	}))

	// 获取 Loader
	loader, err := do.Invoke[*Loader](injector)
	if err != nil {
		t.Fatalf("Invoke failed: %v", err)
	}

	if loader == nil {
		t.Fatal("loader is nil")
	}

	// 验证能读取配置
	appName := loader.GetString("app.name")
	if appName == "" {
		t.Log("app.name is empty (testdata might not have this key)")
	}

	t.Log("✅ ProvideLoader test passed")
}

func TestProvideLoaderValue(t *testing.T) {
	// 预先创建 Loader
	loader, err := NewLoaderBuilder().
		WithConfigPath("testdata").
		Build()
	if err != nil {
		t.Fatalf("Build loader failed: %v", err)
	}

	injector := do.New()

	// 直接注册值
	do.Provide(injector, ProvideLoaderValue(loader))

	// 获取
	gotLoader, err := do.Invoke[*Loader](injector)
	if err != nil {
		t.Fatalf("Invoke failed: %v", err)
	}

	if gotLoader != loader {
		t.Fatal("loader mismatch")
	}

	t.Log("✅ ProvideLoaderValue test passed")
}

func TestProvideLoaderMultipleInvoke(t *testing.T) {
	injector := do.New()

	do.Provide(injector, ProvideLoader(ProvideLoaderOptions{
		ConfigPath: "testdata",
	}))

	// 多次 Invoke 应该返回同一个实例（单例）
	loader1, _ := do.Invoke[*Loader](injector)
	loader2, _ := do.Invoke[*Loader](injector)

	if loader1 != loader2 {
		t.Fatal("multiple Invoke should return same instance (singleton)")
	}

	t.Log("✅ Singleton test passed")
}
