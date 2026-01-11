package testutil

import (
	"testing"

	"github.com/KOMKZ/go-yogan-framework/database"
	"github.com/KOMKZ/go-yogan-framework/logger"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// CLITestContext CLI 测试上下文
// 封装 CLI 测试所需的基础组件
type CLITestContext struct {
	Logger    *zap.Logger
	DBManager *database.Manager
	DBHelper  *DBHelper
}

// CLITestOptions CLI 测试选项
type CLITestOptions struct {
	// AutoMigrate 自动迁移的模型列表
	AutoMigrate []interface{}

	// DBConfig 自定义数据库配置（可选，默认使用 SQLite 内存数据库）
	DBConfig map[string]database.Config

	// Logger 自定义 Logger（可选，默认使用 Development Logger）
	Logger *zap.Logger

	// SetupFunc 自定义初始化函数（可选，在基础初始化完成后调用）
	// 用于创建业务层的 Service、Handler 等
	SetupFunc func(*CLITestContext) error
}

// NewCLITestContext 创建 CLI 测试上下文（一站式初始化）
//
// 使用方式：
//
//	func TestMain(m *testing.M) {
//	    // 1. 创建测试上下文（自动完成所有初始化）
//	    ctx, cleanup := testutil.NewCLITestContext(t, testutil.CLITestOptions{
//	        AutoMigrate: []interface{}{&model.User{}},
//	    })
//	    defer cleanup()
//
//	    // 2. 使用 DBManager 创建 Service
//	    userRepo := user.NewRepositoryImpl(ctx.DBManager.DB("master"))
//	    userService := user.NewService(userRepo)
//
//	    // 3. 运行测试
//	    code := m.Run()
//	    os.Exit(code)
//	}
//
// 优势：
//   - 自动创建 Logger、DBManager、AutoMigrate
//   - 默认使用 SQLite 内存数据库（快速、隔离）
//   - 提供 cleanup 函数自动清理资源
func NewCLITestContext(t *testing.T, opts CLITestOptions) (*CLITestContext, func()) {
	// 1. 创建 Logger（如果未提供）
	zapLogger := opts.Logger
	if zapLogger == nil {
		zapLogger, _ = zap.NewDevelopment()
	}

	// 2. 创建数据库配置（如果未提供，使用默认 SQLite 内存数据库）
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

	// 3. 创建数据库管理器（使用 CtxZapLogger）
	ctxLogger := logger.NewCtxZapLogger("test")
	dbManager, err := database.NewManager(dbConfig, nil, ctxLogger)
	if err != nil {
		t.Fatalf("创建数据库失败: %v", err)
	}

	// 4. 自动迁移表结构
	if len(opts.AutoMigrate) > 0 {
		db := dbManager.DB("master")
		if err := db.AutoMigrate(opts.AutoMigrate...); err != nil {
			dbManager.Close()
			t.Fatalf("迁移表结构失败: %v", err)
		}
	}

	// 5. 创建数据库辅助工具
	dbHelper := NewDBHelper(dbManager.DB("master"))

	// 6. 创建上下文
	ctx := &CLITestContext{
		Logger:    zapLogger,
		DBManager: dbManager,
		DBHelper:  dbHelper,
	}

	// 7. 执行自定义初始化函数
	if opts.SetupFunc != nil {
		if err := opts.SetupFunc(ctx); err != nil {
			dbManager.Close()
			t.Fatalf("自定义初始化失败: %v", err)
		}
	}

	// 8. 返回 cleanup 函数
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

// MustNewCLITestContext 创建 CLI 测试上下文（失败时 fatal）
func MustNewCLITestContext(t *testing.T, opts CLITestOptions) (*CLITestContext, func()) {
	ctx, cleanup := NewCLITestContext(t, opts)
	return ctx, cleanup
}

// SetupTestDB 快速创建测试数据库（极简版）
//
// 适用场景：只需要数据库，不需要其他组件
//
// 使用方式：
//
//	db, cleanup := testutil.SetupTestDB(t, &model.User{})
//	defer cleanup()
func SetupTestDB(t *testing.T, models ...interface{}) (*gorm.DB, func()) {
	// 创建 SQLite 内存数据库
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

	// 自动迁移
	db := dbManager.DB("master")
	if len(models) > 0 {
		if err := db.AutoMigrate(models...); err != nil {
			dbManager.Close()
			t.Fatalf("迁移表结构失败: %v", err)
		}
	}

	cleanup := func() {
		dbManager.Close()
		// CtxZapLogger 的 Sync 通过底层 zap.Logger 实现，无需显式调用
	}

	return db, cleanup
}
