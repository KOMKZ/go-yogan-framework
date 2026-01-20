package testutil

import (
	"testing"

	"github.com/KOMKZ/go-yogan-framework/database"
	"github.com/KOMKZ/go-yogan-framework/logger"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// CLI Test Context
// Encapsulate basic components required for CLI tests
type CLITestContext struct {
	Logger    *zap.Logger
	DBManager *database.Manager
	DBHelper  *DBHelper
}

// CLITestOptions CLI test options
type CLITestOptions struct {
	// AutoMigrate list of models for automatic migration
	AutoMigrate []interface{}

	// DBConfig custom database configuration (optional, defaults to an in-memory SQLite database)
	DBConfig map[string]database.Config

	// Logger custom logger (optional, default uses Development Logger)
	Logger *zap.Logger

	// SetupFunc custom initialization function (optional, called after basic initialization)
	// For creating services, handlers, etc. in the business layer
	SetupFunc func(*CLITestContext) error
}

// NewCLITestContext creates a CLI test context (one-stop initialization)
//
// Usage:
//
//	func TestMain(m *testing.M) {
// // 1. Create test context (auto-complete all initialization)
//	    ctx, cleanup := testutil.NewCLITestContext(t, testutil.CLITestOptions{
//	        AutoMigrate: []interface{}{&model.User{}},
//	    })
//	    defer cleanup()
//
// // 2. Use DBManager to create Service
//	    userRepo := user.NewRepositoryImpl(ctx.DBManager.DB("master"))
//	    userService := user.NewService(userRepo)
//
// // 3. Run tests
//	    code := m.Run()
//	    os.Exit(code)
//	}
//
// Advantages:
// - Automatically create Logger, DBManager, AutoMigrate
// - Default use of SQLite in-memory database (fast, isolated)
// - Provide a cleanup function to automatically clean up resources
func NewCLITestContext(t *testing.T, opts CLITestOptions) (*CLITestContext, func()) {
	// 1. Create Logger (if not provided)
	zapLogger := opts.Logger
	if zapLogger == nil {
		zapLogger, _ = zap.NewDevelopment()
	}

	// 2. Create database configuration (if not provided, use default SQLite in-memory database)
	dbConfig := opts.DBConfig
	if dbConfig == nil {
		dbConfig = map[string]database.Config{
			"master": {
				Driver:       "sqlite",
				DSN:          ":memory:",
				MaxOpenConns: 10,
				MaxIdleConns: 5,
			},
		}
	}

	// 3. Create database manager (using CtxZapLogger)
	ctxLogger := logger.NewCtxZapLogger("test")
	dbManager, err := database.NewManager(dbConfig, nil, ctxLogger)
	if err != nil {
		t.Fatalf("创建数据库失败: %v", err)
	}

	// 4. Automatic table structure migration
	if len(opts.AutoMigrate) > 0 {
		db := dbManager.DB("master")
		if err := db.AutoMigrate(opts.AutoMigrate...); err != nil {
			dbManager.Close()
			t.Fatalf("迁移表结构失败: %v", err)
		}
	}

	// 5. Create database utility
	dbHelper := NewDBHelper(dbManager.DB("master"))

	// 6. Create context
	ctx := &CLITestContext{
		Logger:    zapLogger,
		DBManager: dbManager,
		DBHelper:  dbHelper,
	}

	// 7. Execute custom initialization function
	if opts.SetupFunc != nil {
		if err := opts.SetupFunc(ctx); err != nil {
			dbManager.Close()
			t.Fatalf("自定义初始化失败: %v", err)
		}
	}

	// 8. Return the cleanup function
	cleanup := func() {
		if dbManager != nil {
			dbManager.Close()
		}
		if zapLogger != nil {
			zapLogger.Sync()
		}
	}

	return ctx, cleanup
}

// MustNewCLITestContext Create CLI test context (fatal on failure)
func MustNewCLITestContext(t *testing.T, opts CLITestOptions) (*CLITestContext, func()) {
	ctx, cleanup := NewCLITestContext(t, opts)
	return ctx, cleanup
}

// SetupTestDB Quickly Creates a Minimal Test Database
//
// Applicable scenario: Only the database is required, other components are not needed.
//
// Usage:
//
//	db, cleanup := testutil.SetupTestDB(t, &model.User{})
//	defer cleanup()
func SetupTestDB(t *testing.T, models ...interface{}) (*gorm.DB, func()) {
	// Create an SQLite in-memory database
	dbConfig := map[string]database.Config{
		"master": {
			Driver:       "sqlite",
			DSN:          ":memory:",
			MaxOpenConns: 10,
			MaxIdleConns: 5,
		},
	}

	ctxLogger := logger.NewCtxZapLogger("test")
	dbManager, err := database.NewManager(dbConfig, nil, ctxLogger)
	if err != nil {
		t.Fatalf("创建数据库失败: %v", err)
	}

	// Automatic migration
	db := dbManager.DB("master")
	if len(models) > 0 {
		if err := db.AutoMigrate(models...); err != nil {
			dbManager.Close()
			t.Fatalf("迁移表结构失败: %v", err)
		}
	}

	cleanup := func() {
		dbManager.Close()
		// CtxtZapLogger's Sync is implemented through the underlying zap.Logger, no need to call explicitly
	}

	return db, cleanup
}
