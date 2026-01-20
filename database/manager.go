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

// GormLoggerFactory GORM Logger factory function type
type GormLoggerFactory func(cfg Config) gormlogger.Interface

// Manager database manager (supports multiple instances)
type Manager struct {
	instances      map[string]*gorm.DB
	configs        map[string]Config
	loggerFactory  GormLoggerFactory    // Injected GORM Logger factory
	logger         *logger.CtxZapLogger // Injected business logger (for connecting logs and TraceID)
	otelPlugin     *OtelPlugin          // 🎯 OpenTelemetry plugin
	mu             sync.RWMutex
}

// Create database manager
// configs: database configuration
// loggerFactory: GORM Logger factory function for creating custom loggers (dependency injection)
// logger: business loggger (injected CtxZapLogger instance, must not be nil)
func NewManager(configs map[string]Config, loggerFactory GormLoggerFactory, logger *logger.CtxZapLogger) (*Manager, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	m := &Manager{
		instances:     make(map[string]*gorm.DB),
		configs:       make(map[string]Config),
		loggerFactory: loggerFactory,
		logger:        logger,
		otelPlugin:    nil, // 🎯 To be injected later via SetOtelPlugin
	}

	for name, cfg := range configs {
		// Validate configuration
		if err := cfg.Validate(); err != nil {
			return nil, fmt.Errorf("invalid config for %s: %w", name, err)
		}

		// Open database connection
		db, err := m.openDB(cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to open database %s: %w", name, err)
		}

		// Configure connection pool
		sqlDB, err := db.DB()
		if err != nil {
			return nil, fmt.Errorf("failed to get sql.DB for %s: %w", name, err)
		}

		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
		sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

		m.instances[name] = db
		m.configs[name] = cfg

		m.logger.Debug("Database connection successful",
			zap.String("name", name),
			zap.String("driver", cfg.Driver))
	}

	return m, nil
}

// openDB Open database connection
func (m *Manager) openDB(cfg Config) (*gorm.DB, error) {
	// Select driver
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
	// Configure GORM Logger (via factory function with dependency injection)
	// ====================================
	var gormLogger gormlogger.Interface
	if m.loggerFactory != nil {
		// Use the injected factory function to create a Logger
		gormLogger = m.loggerFactory(cfg)
	} else {
		// When the factory is not injected, use the default silent mode
		gormLogger = gormlogger.Default.LogMode(gormlogger.Silent)
	}

	// Open connection
	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: gormLogger, // Use custom Logger
		NowFunc: func() time.Time {
			return time.Now().Local()
		},
	})

	if err != nil {
		return nil, err
	}

	// 🎯 If there is an OtelPlugin, register it with the database instance
	if m.otelPlugin != nil {
		if err := db.Use(m.otelPlugin); err != nil {
			return nil, fmt.Errorf("failed to use otel plugin: %w", err)
		}
	}

	return db, nil
}

// English: Get specified database instance
func (m *Manager) DB(name string) *gorm.DB {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.instances[name]
}

// Close all database connections
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, db := range m.instances {
		sqlDB, err := db.DB()
		if err != nil {
			m.logger.Error("Failed to get sql.DB sql.DB Failed to get sql.DB",
				zap.String("name", name),
				zap.Error(err))
			continue
		}

		if err := sqlDB.Close(); err != nil {
			m.logger.Error("English: Failed to close database connection",
				zap.String("name", name),
				zap.Error(err))
		} else {
			m.logger.Debug("Database connection closed",
				zap.String("name", name))
		}
	}

	return nil
}

// Implement the samber/do.Shutdownable interface for shutdown functionality
// For automatically closing database connections when the DI container shuts down
func (m *Manager) Shutdown() error {
	return m.Close()
}

// Ping check all database connections
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

// Get database connection pool statistics
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

// SetOtelPlugin Set OpenTelemetry plugin
// Note: Will re-register plugin to all existing database instances
func (m *Manager) SetOtelPlugin(plugin *OtelPlugin) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.otelPlugin = plugin
	
	// 🎯 Read trace_sql and trace_sql_max_len settings from configuration
	// Note: Assumes all database instances use the same OTel configuration
	for _, cfg := range m.configs {
		if cfg.TraceSQL {
			plugin.WithTraceSQL(true)
			m.logger.Debug("✅ GORM OTel trace_sql enabled")
		}
		if cfg.TraceSQLMaxLen > 0 {
			plugin.WithSQLMaxLen(cfg.TraceSQLMaxLen)
			m.logger.Debug("✅ GORM OTel trace_sql_max_len set", zap.Int("max_len", cfg.TraceSQLMaxLen))
		}
		break // Only take the first configuration
	}
	
	// Register the plugin for all existing database instances
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

// SetMetricsPlugin sets the Metrics Plugin for the specified database instance
// dbName: database instance name
// dbMetrics: Database metric collector
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

	// Use the GORMPlugin() method of DBMetrics to get the plugin
	plugin := dbMetrics.GORMPlugin()
	if err := db.Use(plugin); err != nil {
		return fmt.Errorf("failed to register metrics plugin for %s: %w", dbName, err)
	}

	m.logger.Debug("✅ Metrics plugin registered",
		zap.String("db_name", dbName))

	return nil
}

// GetDBNames Retrieve all database instance names
func (m *Manager) GetDBNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.instances))
	for name := range m.instances {
		names = append(names, name)
	}
	return names
}
