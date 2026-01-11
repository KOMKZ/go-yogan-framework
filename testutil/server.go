package testutil

import (
	"testing"

	"github.com/KOMKZ/go-yogan-framework/database"
	"github.com/KOMKZ/go-yogan-framework/redis"
	"github.com/gin-gonic/gin"
)

// TestServer 测试服务器
// 封装完整的应用实例，用于集成测试
type TestServer struct {
	Engine *gin.Engine
	DB     *database.Manager
	Redis  *redis.Manager
}

// TestApp 测试应用接口
// 任何实现了这个接口的应用都可以用于测试
type TestApp interface {
	// RunNonBlocking 非阻塞启动应用（完整启动但不等待关闭信号）
	RunNonBlocking() error

	// GetHTTPServer 获取 HTTP Server（用于测试）
	GetHTTPServer() interface {
		GetEngine() *gin.Engine
	}

	// GetDBManager 获取数据库管理器
	GetDBManager() *database.Manager

	// GetRedisManager 获取 Redis 管理器
	GetRedisManager() *redis.Manager

	// Shutdown 关闭应用
	Shutdown()
}

// NewTestServer 创建测试服务器（优雅版本）
//
// 使用方式：
//
//	// 1. 创建应用实例
//	userApp := app.NewWithConfig(configPath)
//
//	// 2. 注册组件和回调
//	userApp.RegisterComponents(...)
//	userApp.SetupCallbacks(...)
//
//	// 3. 创建测试服务器（自动调用 RunNonBlocking）
//	server, err := testutil.NewTestServer(userApp)
//
// 优势：
//   - 复用 Application.Run() 的完整逻辑
//   - 测试环境和生产环境启动流程完全一致
//   - 代码简洁，易于维护
func NewTestServer(app TestApp) (*TestServer, error) {
	gin.SetMode(gin.TestMode)

	// 执行完整的应用启动流程（非阻塞）
	if err := app.RunNonBlocking(); err != nil {
		return nil, err
	}

	// 获取已初始化的组件
	httpServer := app.GetHTTPServer()
	engine := httpServer.GetEngine()
	dbManager := app.GetDBManager()
	redisManager := app.GetRedisManager()

	return &TestServer{
		Engine: engine,
		DB:     dbManager,
		Redis:  redisManager,
	}, nil
}

// Close 关闭测试服务器
func (ts *TestServer) Close() error {
	// 关闭 Redis 连接
	if ts.Redis != nil {
		if err := ts.Redis.Close(); err != nil {
			return err
		}
	}

	// 关闭数据库连接
	if ts.DB != nil {
		return ts.DB.Close()
	}
	return nil
}

// MustNewTestServer 创建测试服务器（失败时 panic）
func MustNewTestServer(t *testing.T, app TestApp) *TestServer {
	server, err := NewTestServer(app)
	if err != nil {
		t.Fatalf("创建测试服务器失败: %v", err)
	}
	return server
}
