package database

import (
	"context"
	"fmt"

	"github.com/KOMKZ/go-yogan-framework/component"
	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/KOMKZ/go-yogan-framework/registry"
	"github.com/KOMKZ/go-yogan-framework/telemetry"
	"go.uber.org/zap"
	gormlogger "gorm.io/gorm/logger"
)

// Component 数据库组件
//
// 实现 component.Component 接口，提供数据库管理能力
// 依赖：config, logger
// 可选依赖：telemetry（在 Start 阶段动态注入）
type Component struct {
	manager  *Manager
	registry *registry.Registry   // 🎯 使用具体类型，支持泛型方法
	logger   *logger.CtxZapLogger // 🎯 组件统一使用字段保存 logger
}

// NewComponent 创建数据库组件
func NewComponent() *Component {
	return &Component{}
}

// SetRegistry 设置 Registry（由框架调用）
func (c *Component) SetRegistry(r *registry.Registry) {
	c.registry = r
}

// Name 组件名称
func (c *Component) Name() string {
	return component.ComponentDatabase
}

// DependsOn 数据库组件依赖配置、日志，可选依赖 Telemetry
func (c *Component) DependsOn() []string {
	return []string{
		component.ComponentConfig,
		component.ComponentLogger,
		"optional:" + component.ComponentTelemetry, // 🎯 可选依赖 Telemetry
	}
}

// Init 初始化数据库管理器
//
// 🎯 简化后的实现：直接从 ConfigLoader 读取配置
func (c *Component) Init(ctx context.Context, loader component.ConfigLoader) error {
	// 🎯 统一在 Init 开始时保存 logger 到字段
	c.logger = logger.GetLogger("yogan")
	c.logger.DebugCtx(ctx, "🔧 Database 组件开始初始化...")

	// 直接从 ConfigLoader 读取数据库配置！
	var dbConfigs map[string]Config
	if err := loader.Unmarshal("database.connections", &dbConfigs); err != nil {
		return fmt.Errorf("读取数据库配置失败: %w", err)
	}

	// 如果未配置，跳过初始化
	if len(dbConfigs) == 0 {
		c.logger.DebugCtx(ctx, "未配置数据库，跳过初始化")
		return nil
	}

	// 创建 GORM Logger 工厂函数
	gormLoggerFactory := func(dbCfg Config) gormlogger.Interface {
		if dbCfg.EnableLog {
			loggerCfg := logger.DefaultGormLoggerConfig()
			loggerCfg.SlowThreshold = dbCfg.SlowThreshold
			loggerCfg.LogLevel = gormlogger.Info
			loggerCfg.EnableAudit = dbCfg.EnableAudit
			return logger.NewGormLogger(loggerCfg)
		}
		return gormlogger.Default.LogMode(gormlogger.Silent)
	}

	// 创建数据库管理器（直接传递 CtxZapLogger）
	manager, err := NewManager(dbConfigs, gormLoggerFactory, c.logger)
	if err != nil {
		return fmt.Errorf("创建数据库管理器失败: %w", err)
	}

	c.manager = manager
	c.logger.DebugCtx(ctx, "✅ 数据库初始化成功")
	return nil
}

// Start 启动数据库组件
// 🎯 在此阶段注入 OpenTelemetry 插件（如果 Telemetry 组件存在）
func (c *Component) Start(ctx context.Context) error {
	if c.manager == nil {
		return nil
	}

	// 🎯 尝试从 Registry 获取 Telemetry 组件并注入 TracerProvider
	c.injectTracerProvider(ctx)

	// 🎯 尝试从 Telemetry 组件获取 MetricsManager 并注入
	c.injectMetricsManager(ctx)

	return nil
}

// injectTracerProvider 从 Telemetry 组件获取 TracerProvider 并注入到 GORM
func (c *Component) injectTracerProvider(ctx context.Context) {
	if c.registry == nil {
		return
	}

	// 🎯 使用通用注入器
	injector := registry.NewInjector(c.registry, c.logger)
	registry.Inject(injector, ctx, component.ComponentTelemetry,
		func(tc *telemetry.Component) bool { return tc.IsEnabled() },
		func(tc *telemetry.Component) {
			tp := tc.GetTracerProvider()
			if tp == nil {
				c.logger.WarnCtx(ctx, "TracerProvider is nil")
				return
			}

			// 创建 OtelPlugin 并注入到 Manager
			otelPlugin := NewOtelPlugin(tp)
			if err := c.manager.SetOtelPlugin(otelPlugin); err != nil {
				c.logger.ErrorCtx(ctx, "Failed to inject TracerProvider into GORM", zap.Error(err))
				return
			}

			c.logger.DebugCtx(ctx, "✅ TracerProvider injected into GORM")
		},
	)
}

// injectMetricsManager 从 Telemetry 组件获取 MetricsManager 并注入到 GORM
func (c *Component) injectMetricsManager(ctx context.Context) {
	if c.registry == nil {
		return
	}

	// 🎯 使用通用注入器
	injector := registry.NewInjector(c.registry, c.logger)
	registry.Inject(injector, ctx, component.ComponentTelemetry,
		func(tc *telemetry.Component) bool {
			// 检查 Telemetry 启用 + MetricsManager 可用 + DB Metrics 启用
			if !tc.IsEnabled() {
				return false
			}
			mm := tc.GetMetricsManager()
			return mm != nil && mm.IsDBMetricsEnabled()
		},
		func(tc *telemetry.Component) {
			// 遍历所有数据库实例，为每个实例创建并注入 Metrics Plugin
			dbNames := c.manager.GetDBNames()
			for _, dbName := range dbNames {
				db := c.manager.DB(dbName)
				if db == nil {
					continue
				}

				// 创建 DBMetrics（默认配置）
				dbMetrics, err := NewDBMetrics(db, false, 1.0)
				if err != nil {
					c.logger.ErrorCtx(ctx, "Failed to create DBMetrics",
						zap.String("db_name", dbName),
						zap.Error(err))
					continue
				}

				// 注入到 Manager
				if err := c.manager.SetMetricsPlugin(dbName, dbMetrics); err != nil {
					c.logger.ErrorCtx(ctx, "Failed to inject MetricsPlugin into GORM",
						zap.String("db_name", dbName),
						zap.Error(err))
					continue
				}

				c.logger.DebugCtx(ctx, "✅ MetricsPlugin injected into GORM",
					zap.String("db_name", dbName))
			}
		},
	)
}

// Stop 停止数据库组件（关闭连接）
func (c *Component) Stop(ctx context.Context) error {
	if c.manager != nil {
		if err := c.manager.Close(); err != nil {
			return fmt.Errorf("关闭数据库连接失败: %w", err)
		}
	}
	return nil
}

// GetManager 获取数据库管理器
func (c *Component) GetManager() *Manager {
	return c.manager
}

// GetHealthChecker 获取健康检查器
// 实现 component.HealthCheckProvider 接口
func (c *Component) GetHealthChecker() component.HealthChecker {
	if c.manager == nil {
		return nil
	}
	return NewHealthChecker(c.manager)
}
