package testutil

import (
	"fmt"
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

// InProcessTestApp exposes the complete HTTP application lifecycle without
// requiring a TCP listener.
type InProcessTestApp interface {
	RunInProcess() error

	GetHTTPServer() interface {
		GetEngine() *gin.Engine
	}

	GetDBManager() *database.Manager
	GetRedisManager() *redis.Manager
	Shutdown()
}

// Create test server (elegant version)
//
// Usage:
//
// // 1. Create application instance
//
//	userApp := app.NewWithConfig(configPath)
//
// // 2. Register components and callbacks
//
//	userApp.RegisterComponents(...)
//	userApp.SetupCallbacks(...)
//
// // 3. Create test server (automatically calls RunNonBlocking)
//
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

	return initializedTestServer(app.GetHTTPServer(), app.GetDBManager(), app.GetRedisManager())
}

// NewInProcessTestServer initializes the real application, DI graph, routes,
// and lifecycle callbacks while serving requests directly through Gin. It is
// intended for HTTP integration tests that need production configuration but
// must not reserve a TCP port.
func NewInProcessTestServer(app InProcessTestApp) (*TestServer, error) {
	gin.SetMode(gin.TestMode)

	if err := app.RunInProcess(); err != nil {
		return nil, err
	}

	return initializedTestServer(app.GetHTTPServer(), app.GetDBManager(), app.GetRedisManager())
}

func initializedTestServer(
	httpServer interface{ GetEngine() *gin.Engine },
	dbManager *database.Manager,
	redisManager *redis.Manager,
) (*TestServer, error) {
	if httpServer == nil || httpServer.GetEngine() == nil {
		return nil, fmt.Errorf("test application did not initialize an HTTP server")
	}

	return &TestServer{
		Engine: httpServer.GetEngine(),
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
		t.Fatalf("Failed to create test server: %v", err)
	}
	return server
}
