package database

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// GormLoggerFactory GORM Logger 工厂函数类型
type GormLoggerFactory func(cfg Config) gormlogger.Interface

// Manager 数据库管理器（支持多实例）
type Manager struct {
	instances      map[string]*gorm.DB
	configs        map[string]Config
	loggerFactory  GormLoggerFactory    // 注入的 GORM Logger 工厂
	logger         *logger.CtxZapLogger // 注入的业务日志器（用于连接日志和 TraceID）
	otelPlugin     *OtelPlugin          // 🎯 OpenTelemetry 插件
	mu             sync.RWMutex
}

// NewManager 创建数据库管理器
// configs: 数据库配置
// loggerFactory: GORM Logger 工厂函数，用于创建自定义日志器（依赖注入）
// logger: 业务日志器（注入的 CtxZapLogger 实例，不能为 nil）
func NewManager(configs map[string]Config, loggerFactory GormLoggerFactory, logger *logger.CtxZapLogger) (*Manager, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	m := &Manager{
		instances:     make(map[string]*gorm.DB),
		configs:       make(map[string]Config),
		loggerFactory: loggerFactory,
		logger:        logger,
		otelPlugin:    nil, // 🎯 稍后通过 SetOtelPlugin 注入
	}

	for name, cfg := range configs {
		// 验证配置
		if err := cfg.Validate(); err != nil {
			return nil, fmt.Errorf("invalid config for %s: %w", name, err)
		}

		// 打开数据库连接
		db, err := m.openDB(cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to open database %s: %w", name, err)
		}

		// 配置连接池
		sqlDB, err := db.DB()
		if err != nil {
			return nil, fmt.Errorf("failed to get sql.DB for %s: %w", name, err)
		}

		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
		sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

		m.instances[name] = db
		m.configs[name] = cfg

		m.logger.Debug("数据库连接成功",
			zap.String("name", name),
			zap.String("driver", cfg.Driver))
	}

	return m, nil
}

// openDB 打开数据库连接
func (m *Manager) openDB(cfg Config) (*gorm.DB, error) {
	// 选择驱动
	var dialector gorm.Dialector
	switch cfg.Driver {
	case "mysql":
		dialector = mysql.Open(cfg.DSN)
	case "postgres":
		dialector = postgres.Open(cfg.DSN)
	case "sqlite":
		dialector = sqlite.Open(cfg.DSN)
	default:
		return nil, fmt.Errorf("unsupported driver: %s", cfg.Driver)
	}

	// ====================================
	// 配置 GORM Logger（通过依赖注入的工厂函数）
	// ====================================
	var gormLogger gormlogger.Interface
	if m.loggerFactory != nil {
		// 使用注入的工厂函数创建 Logger
		gormLogger = m.loggerFactory(cfg)
	} else {
		// 未注入工厂时，使用默认的静默模式
		gormLogger = gormlogger.Default.LogMode(gormlogger.Silent)
	}

	// 打开连接
	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: gormLogger, // 使用自定义 Logger
		NowFunc: func() time.Time {
			return time.Now().Local()
		},
	})

	if err != nil {
		return nil, err
	}

	// 🎯 如果有 OtelPlugin，注册到数据库实例
	if m.otelPlugin != nil {
		if err := db.Use(m.otelPlugin); err != nil {
			return nil, fmt.Errorf("failed to use otel plugin: %w", err)
		}
	}

	return db, nil
}

// DB 获取指定数据库实例
func (m *Manager) DB(name string) *gorm.DB {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.instances[name]
}

// Close 关闭所有数据库连接
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, db := range m.instances {
		sqlDB, err := db.DB()
		if err != nil {
			m.logger.Error("获取 sql.DB 失败",
				zap.String("name", name),
				zap.Error(err))
			continue
		}

		if err := sqlDB.Close(); err != nil {
			m.logger.Error("关闭数据库连接失败",
				zap.String("name", name),
				zap.Error(err))
		} else {
			m.logger.Debug("数据库连接已关闭",
				zap.String("name", name))
		}
	}

	return nil
}

// Shutdown 实现 samber/do.Shutdownable 接口
// 用于在 DI 容器关闭时自动关闭数据库连接
func (m *Manager) Shutdown() error {
	return m.Close()
}

// Ping 检查所有数据库连接
func (m *Manager) Ping() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for name, db := range m.instances {
		sqlDB, err := db.DB()
		if err != nil {
			return fmt.Errorf("failed to get sql.DB for %s: %w", name, err)
		}

		if err := sqlDB.Ping(); err != nil {
			return fmt.Errorf("ping failed for %s: %w", name, err)
		}
	}

	return nil
}

// Stats 获取数据库连接池统计信息
func (m *Manager) Stats(name string) (sql.DBStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	db, ok := m.instances[name]
	if !ok {
		return sql.DBStats{}, fmt.Errorf("database %s not found", name)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return sql.DBStats{}, err
	}

	return sqlDB.Stats(), nil
}

// SetOtelPlugin 设置 OpenTelemetry 插件
// 注意：会重新注册插件到所有已存在的数据库实例
func (m *Manager) SetOtelPlugin(plugin *OtelPlugin) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.otelPlugin = plugin
	
	// 🎯 从配置中读取 trace_sql 和 trace_sql_max_len 设置
	// 注意：假设所有数据库实例使用相同的 OTel 配置
	for _, cfg := range m.configs {
		if cfg.TraceSQL {
			plugin.WithTraceSQL(true)
			m.logger.Debug("✅ GORM OTel trace_sql enabled")
		}
		if cfg.TraceSQLMaxLen > 0 {
			plugin.WithSQLMaxLen(cfg.TraceSQLMaxLen)
			m.logger.Debug("✅ GORM OTel trace_sql_max_len set", zap.Int("max_len", cfg.TraceSQLMaxLen))
		}
		break // 只取第一个配置
	}
	
	// 为所有已存在的数据库实例注册插件
	for name, db := range m.instances {
		if err := db.Use(plugin); err != nil {
			m.logger.Error("Failed to register otel plugin",
				zap.String("instance", name),
				zap.Error(err))
			return fmt.Errorf("failed to register otel plugin for %s: %w", name, err)
		}
		m.logger.Debug("OTel plugin registered",
			zap.String("instance", name))
	}
	
	return nil
}

// SetMetricsPlugin 为指定数据库实例设置 Metrics Plugin
// dbName: 数据库实例名称
// dbMetrics: 数据库指标收集器
func (m *Manager) SetMetricsPlugin(dbName string, dbMetrics *DBMetrics) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	db, exists := m.instances[dbName]
	if !exists {
		return fmt.Errorf("database instance %s not found", dbName)
	}

	if dbMetrics == nil {
		return fmt.Errorf("dbMetrics is nil")
	}

	// 使用 DBMetrics 的 GORMPlugin() 方法获取 plugin
	plugin := dbMetrics.GORMPlugin()
	if err := db.Use(plugin); err != nil {
		return fmt.Errorf("failed to register metrics plugin for %s: %w", dbName, err)
	}

	m.logger.Debug("✅ Metrics plugin registered",
		zap.String("db_name", dbName))

	return nil
}

// GetDBNames 获取所有数据库实例名称
func (m *Manager) GetDBNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.instances))
	for name := range m.instances {
		names = append(names, name)
	}
	return names
}
