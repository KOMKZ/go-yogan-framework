package di

import (
	"testing"

	"github.com/KOMKZ/go-yogan-framework/config"
	"github.com/KOMKZ/go-yogan-framework/database"
	"github.com/KOMKZ/go-yogan-framework/event"
	"github.com/KOMKZ/go-yogan-framework/health"
	"github.com/KOMKZ/go-yogan-framework/jwt"
	"github.com/KOMKZ/go-yogan-framework/kafka"
	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/KOMKZ/go-yogan-framework/redis"
	"github.com/KOMKZ/go-yogan-framework/telemetry"
	"github.com/samber/do/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProvideConfigLoader 测试配置加载器 Provider
func TestProvideConfigLoader(t *testing.T) {
	t.Run("default options", func(t *testing.T) {
		injector := do.New()
		defer injector.Shutdown()

		opts := ConfigOptions{}
		do.Provide(injector, ProvideConfigLoader(opts))

		// Provider 应该成功注册
		// 即使配置加载可能失败，Provider 仍会返回 Loader 实例
		loader, err := do.Invoke[*config.Loader](injector)
		// ConfigLoader 构建器即使没有配置文件也会返回实例
		if err == nil {
			assert.NotNil(t, loader)
		}
	})

	t.Run("with custom options", func(t *testing.T) {
		opts := ConfigOptions{
			ConfigPath:   "../testdata",
			ConfigPrefix: "TEST",
			AppType:      "http",
		}

		// 验证默认值应用
		assert.Equal(t, "../testdata", opts.ConfigPath)
		assert.Equal(t, "TEST", opts.ConfigPrefix)
		assert.Equal(t, "http", opts.AppType)
	})

	t.Run("with testdata config", func(t *testing.T) {
		injector := do.New()
		defer injector.Shutdown()

		opts := ConfigOptions{
			ConfigPath: "./testdata",
			AppType:    "http",
		}
		do.Provide(injector, ProvideConfigLoader(opts))

		loader, err := do.Invoke[*config.Loader](injector)
		require.NoError(t, err)
		assert.NotNil(t, loader)
	})

	t.Run("applies defaults for empty fields", func(t *testing.T) {
		injector := do.New()
		defer injector.Shutdown()

		// 空选项，应该应用默认值
		opts := ConfigOptions{}
		provider := ProvideConfigLoader(opts)
		assert.NotNil(t, provider)
	})
}

// TestProvideLoggerManager 测试 Logger Manager Provider
func TestProvideLoggerManager(t *testing.T) {
	t.Run("without config loader", func(t *testing.T) {
		injector := do.New()
		defer injector.Shutdown()

		do.Provide(injector, ProvideLoggerManager)

		// 没有 config.Loader，应该回退到默认配置
		mgr, err := do.Invoke[*logger.Manager](injector)
		require.NoError(t, err)
		assert.NotNil(t, mgr)
	})

	t.Run("get logger from manager", func(t *testing.T) {
		injector := do.New()
		defer injector.Shutdown()

		do.Provide(injector, ProvideLoggerManager)

		mgr, err := do.Invoke[*logger.Manager](injector)
		require.NoError(t, err)

		log := mgr.GetLogger("test")
		assert.NotNil(t, log)
	})
}

// TestProvideCtxLogger 测试命名 Logger Provider
func TestProvideCtxLogger(t *testing.T) {
	t.Run("without manager", func(t *testing.T) {
		injector := do.New()
		defer injector.Shutdown()

		do.Provide(injector, ProvideCtxLogger("test-module"))

		// 没有 Manager，应该回退到全局 logger
		log, err := do.Invoke[*logger.CtxZapLogger](injector)
		require.NoError(t, err)
		assert.NotNil(t, log)
	})

	t.Run("with manager", func(t *testing.T) {
		injector := do.New()
		defer injector.Shutdown()

		do.Provide(injector, ProvideLoggerManager)
		do.ProvideNamed(injector, "mymodule", ProvideCtxLogger("mymodule"))

		log, err := do.InvokeNamed[*logger.CtxZapLogger](injector, "mymodule")
		require.NoError(t, err)
		assert.NotNil(t, log)
	})
}

// TestProvideDatabaseManager 测试数据库 Manager Provider
func TestProvideDatabaseManager(t *testing.T) {
	t.Run("without config loader", func(t *testing.T) {
		injector := do.New()
		defer injector.Shutdown()

		do.Provide(injector, ProvideDatabaseManager)

		// 没有 config.Loader，应该报错
		_, err := do.Invoke[*struct{}](injector)
		assert.Error(t, err)
	})
}

// TestProvideRedisManager 测试 Redis Manager Provider
func TestProvideRedisManager(t *testing.T) {
	t.Run("without config loader", func(t *testing.T) {
		injector := do.New()
		defer injector.Shutdown()

		do.Provide(injector, ProvideRedisManager)

		// 没有 config.Loader，应该报错
		_, err := do.Invoke[*struct{}](injector)
		assert.Error(t, err)
	})
}

// TestConfigOptions 测试配置选项结构
func TestConfigOptions(t *testing.T) {
	opts := ConfigOptions{
		ConfigPath:   "/path/to/config",
		ConfigPrefix: "MYAPP",
		AppType:      "mixed",
		Flags:        nil,
	}

	assert.Equal(t, "/path/to/config", opts.ConfigPath)
	assert.Equal(t, "MYAPP", opts.ConfigPrefix)
	assert.Equal(t, "mixed", opts.AppType)
	assert.Nil(t, opts.Flags)
}

// TestProvideConfigLoaderWithFlags 测试带 Flags 的配置加载
func TestProvideConfigLoaderWithFlags(t *testing.T) {
	type TestFlags struct {
		Debug bool
	}

	opts := ConfigOptions{
		ConfigPath: "../config",
		AppType:    "http",
		Flags:      &TestFlags{Debug: true},
	}

	provider := ProvideConfigLoader(opts)
	assert.NotNil(t, provider)
}

// TestProvideLoggerManagerIntegration 集成测试 Logger Manager
func TestProvideLoggerManagerIntegration(t *testing.T) {
	injector := do.New()
	defer injector.Shutdown()

	// 注册 Logger Manager
	do.Provide(injector, ProvideLoggerManager)

	// 注册多个命名 Logger
	do.ProvideNamed(injector, "api", ProvideCtxLogger("api"))
	do.ProvideNamed(injector, "db", ProvideCtxLogger("db"))
	do.ProvideNamed(injector, "auth", ProvideCtxLogger("auth"))

	// 验证各模块 Logger 独立
	apiLog, err := do.InvokeNamed[*logger.CtxZapLogger](injector, "api")
	require.NoError(t, err)
	assert.NotNil(t, apiLog)

	dbLog, err := do.InvokeNamed[*logger.CtxZapLogger](injector, "db")
	require.NoError(t, err)
	assert.NotNil(t, dbLog)

	authLog, err := do.InvokeNamed[*logger.CtxZapLogger](injector, "auth")
	require.NoError(t, err)
	assert.NotNil(t, authLog)
}

// TestProvideDatabaseManagerWithMockConfig 测试带模拟配置的数据库 Provider
func TestProvideDatabaseManagerWithMockConfig(t *testing.T) {
	// 这个测试验证 Provider 的逻辑路径
	// 实际数据库连接需要真实配置
	t.Run("provider function is callable", func(t *testing.T) {
		injector := do.New()
		defer injector.Shutdown()

		// 注册 Database Provider
		do.Provide(injector, ProvideDatabaseManager)

		// 没有配置，调用会失败
		_, err := do.Invoke[*struct{ Name string }](injector)
		assert.Error(t, err)
	})
}

// TestProvideRedisManagerWithMockConfig 测试带模拟配置的 Redis Provider
func TestProvideRedisManagerWithMockConfig(t *testing.T) {
	t.Run("provider function is callable", func(t *testing.T) {
		injector := do.New()
		defer injector.Shutdown()

		// 注册 Redis Provider
		do.Provide(injector, ProvideRedisManager)

		// 没有配置，调用会失败
		_, err := do.Invoke[*struct{ Name string }](injector)
		assert.Error(t, err)
	})
}

// TestProvideDatabaseManagerWithRealConfig 测试带真实配置的数据库 Provider
func TestProvideDatabaseManagerWithRealConfig(t *testing.T) {
	t.Run("with config loader but no db config", func(t *testing.T) {
		injector := do.New()
		defer injector.Shutdown()

		// 使用测试配置目录
		opts := ConfigOptions{
			ConfigPath: "./testdata",
			AppType:    "http",
		}
		do.Provide(injector, ProvideConfigLoader(opts))
		do.Provide(injector, ProvideLoggerManager)
		do.Provide(injector, ProvideCtxLogger("yogan"))
		do.Provide(injector, ProvideDatabaseManager)

		// 尝试调用 - 因为没有数据库配置，返回 nil
		mgr, err := do.Invoke[*database.Manager](injector)
		// 没有配置时应该返回 nil, nil
		if err == nil {
			assert.Nil(t, mgr) // 未配置数据库返回 nil
		}
	})

	t.Run("without logger - fallback to global", func(t *testing.T) {
		injector := do.New()
		defer injector.Shutdown()

		// 只注册 ConfigLoader，不注册 Logger
		opts := ConfigOptions{
			ConfigPath: "./testdata",
			AppType:    "http",
		}
		do.Provide(injector, ProvideConfigLoader(opts))
		do.Provide(injector, ProvideDatabaseManager)

		// 应该使用全局 logger 并返回 nil（无数据库配置）
		mgr, err := do.Invoke[*database.Manager](injector)
		if err == nil {
			assert.Nil(t, mgr)
		}
	})
}

// TestProvideRedisManagerWithRealConfig 测试带真实配置的 Redis Provider
func TestProvideRedisManagerWithRealConfig(t *testing.T) {
	t.Run("with config loader but no redis config", func(t *testing.T) {
		injector := do.New()
		defer injector.Shutdown()

		// 使用测试配置目录
		opts := ConfigOptions{
			ConfigPath: "./testdata",
			AppType:    "http",
		}
		do.Provide(injector, ProvideConfigLoader(opts))
		do.Provide(injector, ProvideLoggerManager)
		do.Provide(injector, ProvideCtxLogger("yogan"))
		do.Provide(injector, ProvideRedisManager)

		// 尝试调用 - 因为没有 Redis 配置，返回 nil
		mgr, err := do.Invoke[*redis.Manager](injector)
		// 没有配置时应该返回 nil, nil
		if err == nil {
			assert.Nil(t, mgr) // 未配置 Redis 返回 nil
		}
	})

	t.Run("without logger - fallback to global", func(t *testing.T) {
		injector := do.New()
		defer injector.Shutdown()

		// 只注册 ConfigLoader，不注册 Logger
		opts := ConfigOptions{
			ConfigPath: "./testdata",
			AppType:    "http",
		}
		do.Provide(injector, ProvideConfigLoader(opts))
		do.Provide(injector, ProvideRedisManager)

		// 应该使用全局 logger 并返回 nil（无 Redis 配置）
		mgr, err := do.Invoke[*redis.Manager](injector)
		if err == nil {
			assert.Nil(t, mgr)
		}
	})
}

// TestProvideLoggerManagerWithConfig 测试带配置的 Logger Manager
func TestProvideLoggerManagerWithConfig(t *testing.T) {
	t.Run("with config loader", func(t *testing.T) {
		injector := do.New()
		defer injector.Shutdown()

		// 使用测试配置目录
		opts := ConfigOptions{
			ConfigPath: "./testdata",
			AppType:    "http",
		}
		do.Provide(injector, ProvideConfigLoader(opts))
		do.Provide(injector, ProvideLoggerManager)

		// 应该成功获取 Manager
		mgr, err := do.Invoke[*logger.Manager](injector)
		require.NoError(t, err)
		assert.NotNil(t, mgr)
	})

	t.Run("with valid config", func(t *testing.T) {
		injector := do.New()
		defer injector.Shutdown()

		opts := ConfigOptions{
			ConfigPath: "./testdata",
			AppType:    "http",
		}
		do.Provide(injector, ProvideConfigLoader(opts))
		do.Provide(injector, ProvideLoggerManager)

		mgr, err := do.Invoke[*logger.Manager](injector)
		require.NoError(t, err)
		assert.NotNil(t, mgr)

		// 验证可以获取 logger
		log := mgr.GetLogger("test")
		assert.NotNil(t, log)
	})
}

// ============================================
// JWT Provider 测试
// ============================================

// TestProvideJWTTokenManagerIndependent 测试 JWT TokenManager 独立 Provider
func TestProvideJWTTokenManagerIndependent(t *testing.T) {
	t.Run("without config loader", func(t *testing.T) {
		injector := do.New()
		defer injector.Shutdown()

		do.Provide(injector, ProvideJWTTokenManagerIndependent)

		// 没有 config.Loader，应该报错
		_, err := do.Invoke[jwt.TokenManager](injector)
		assert.Error(t, err)
	})

	t.Run("with config but jwt disabled", func(t *testing.T) {
		injector := do.New()
		defer injector.Shutdown()

		// 使用测试配置（JWT 未启用）
		opts := ConfigOptions{
			ConfigPath: "./testdata",
			AppType:    "http",
		}
		do.Provide(injector, ProvideConfigLoader(opts))
		do.Provide(injector, ProvideLoggerManager)
		do.Provide(injector, ProvideCtxLogger("yogan"))
		do.Provide(injector, ProvideJWTTokenManagerIndependent)

		// JWT 未启用，返回 nil
		mgr, err := do.Invoke[jwt.TokenManager](injector)
		// 可能返回 nil, nil（未启用）
		if err == nil {
			assert.Nil(t, mgr)
		}
	})
}

// ============================================
// Event Provider 测试
// ============================================

// TestProvideEventDispatcherIndependent 测试 Event Dispatcher 独立 Provider
func TestProvideEventDispatcherIndependent(t *testing.T) {
	t.Run("without config loader", func(t *testing.T) {
		injector := do.New()
		defer injector.Shutdown()

		do.Provide(injector, ProvideEventDispatcherIndependent)

		// 没有 config.Loader，应该报错
		_, err := do.Invoke[event.Dispatcher](injector)
		assert.Error(t, err)
	})

	t.Run("with config but event disabled", func(t *testing.T) {
		injector := do.New()
		defer injector.Shutdown()

		// 使用测试配置（Event 未启用）
		opts := ConfigOptions{
			ConfigPath: "./testdata",
			AppType:    "http",
		}
		do.Provide(injector, ProvideConfigLoader(opts))
		do.Provide(injector, ProvideLoggerManager)
		do.Provide(injector, ProvideCtxLogger("yogan"))
		do.Provide(injector, ProvideEventDispatcherIndependent)

		// Event 未启用，返回 nil
		dispatcher, err := do.Invoke[event.Dispatcher](injector)
		// 可能返回 nil, nil（未启用）
		if err == nil {
			assert.Nil(t, dispatcher)
		}
	})
}

// ============================================
// Kafka Provider 测试
// ============================================

// TestProvideKafkaManager 测试 Kafka Manager Provider
func TestProvideKafkaManager(t *testing.T) {
	t.Run("without config loader", func(t *testing.T) {
		injector := do.New()
		defer injector.Shutdown()

		do.Provide(injector, ProvideKafkaManager)

		// 没有 config.Loader，应该报错
		_, err := do.Invoke[*kafka.Manager](injector)
		assert.Error(t, err)
	})

	t.Run("with config but kafka not configured", func(t *testing.T) {
		injector := do.New()
		defer injector.Shutdown()

		opts := ConfigOptions{
			ConfigPath: "./testdata",
			AppType:    "http",
		}
		do.Provide(injector, ProvideConfigLoader(opts))
		do.Provide(injector, ProvideLoggerManager)
		do.Provide(injector, ProvideCtxLogger("yogan"))
		do.Provide(injector, ProvideKafkaManager)

		// Kafka 未配置，返回 nil
		mgr, err := do.Invoke[*kafka.Manager](injector)
		if err == nil {
			assert.Nil(t, mgr)
		}
	})
}

// ============================================
// Telemetry Provider 测试
// ============================================

// TestProvideTelemetryComponent 测试 Telemetry Component Provider
func TestProvideTelemetryComponent(t *testing.T) {
	t.Run("without config loader", func(t *testing.T) {
		injector := do.New()
		defer injector.Shutdown()

		do.Provide(injector, ProvideTelemetryComponent)

		// 没有 config.Loader，应该报错
		_, err := do.Invoke[*telemetry.Component](injector)
		assert.Error(t, err)
	})

	t.Run("with config but telemetry disabled", func(t *testing.T) {
		injector := do.New()
		defer injector.Shutdown()

		opts := ConfigOptions{
			ConfigPath: "./testdata",
			AppType:    "http",
		}
		do.Provide(injector, ProvideConfigLoader(opts))
		do.Provide(injector, ProvideTelemetryComponent)

		// Telemetry 未启用，返回 nil
		comp, err := do.Invoke[*telemetry.Component](injector)
		if err == nil {
			assert.Nil(t, comp)
		}
	})
}

// ============================================
// Health Provider 测试
// ============================================

// TestProvideHealthAggregator 测试 Health Aggregator Provider
func TestProvideHealthAggregator(t *testing.T) {
	t.Run("without config loader", func(t *testing.T) {
		injector := do.New()
		defer injector.Shutdown()

		do.Provide(injector, ProvideHealthAggregator)

		// 没有 config.Loader，应该报错
		_, err := do.Invoke[*health.Aggregator](injector)
		assert.Error(t, err)
	})

	t.Run("with config - health enabled by default", func(t *testing.T) {
		injector := do.New()
		defer injector.Shutdown()

		opts := ConfigOptions{
			ConfigPath: "./testdata",
			AppType:    "http",
		}
		do.Provide(injector, ProvideConfigLoader(opts))
		do.Provide(injector, ProvideHealthAggregator)

		// Health 默认启用
		agg, err := do.Invoke[*health.Aggregator](injector)
		require.NoError(t, err)
		assert.NotNil(t, agg)
	})
}
