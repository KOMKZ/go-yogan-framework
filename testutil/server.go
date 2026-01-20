package testutil

import (
	"testing"

	"github.com/KOMKZ/go-yogan-framework/database"
	"github.com/KOMKZ/go-yogan-framework/redis"
	"github.com/gin-gonic/gin"
)

// TestServer test server
// Encapsulate the complete application instance for integration testing
type TestServer struct {
	Engine *gin.Engine
	DB     *database.Manager
	Redis  *redis.Manager
}

// TestApp interface testing
// Any application that implements this interface can be used for testing
type TestApp interface {
	// RunNonBlocking Start the application non-blockingly (fully start but do not wait for shutdown signal)
	RunNonBlocking() error

	// GetHTTPServer obtain HTTP server (for testing)
	GetHTTPServer() interface {
		GetEngine() *gin.Engine
	}

	// GetDBManager Obtain database manager
	GetDBManager() *database.Manager

	// GetRedisManager Get Redis manager
	GetRedisManager() *redis.Manager

	// Shut down application
	Shutdown()
}

// Create test server (elegant version)
//
// Usage:
//
// // 1. Create application instance
//	userApp := app.NewWithConfig(configPath)
//
// // 2. Register components and callbacks
//	userApp.RegisterComponents(...)
//	userApp.SetupCallbacks(...)
//
// // 3. Create test server (automatically calls RunNonBlocking)
//	server, err := testutil.NewTestServer(userApp)
//
// Advantages:
// - Reuse the complete logic of Application.Run()
// - The startup process for the test environment is identical to that of the production environment
// code is concise and easy to maintain
func NewTestServer(app TestApp) (*TestServer, error) {
	gin.SetMode(gin.TestMode)

	// Execute the full application startup process (non-blocking)
	if err := app.RunNonBlocking(); err != nil {
		return nil, err
	}

	// Get initialized components
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

// Close the test server
func (ts *TestServer) Close() error {
	// Close Redis connection
	if ts.Redis != nil {
		if err := ts.Redis.Close(); err != nil {
			return err
		}
	}

	// Close database connection
	if ts.DB != nil {
		return ts.DB.Close()
	}
	return nil
}

// MustNewTestServer Create a test server (panic on failure)
func MustNewTestServer(t *testing.T, app TestApp) *TestServer {
	server, err := NewTestServer(app)
	if err != nil {
		t.Fatalf("创建测试服务器失败: %v", err)
	}
	return server
}
