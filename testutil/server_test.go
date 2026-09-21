package testutil_test

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/KOMKZ/go-yogan-framework/application"
	"github.com/KOMKZ/go-yogan-framework/database"
	"github.com/KOMKZ/go-yogan-framework/redis"
	"github.com/KOMKZ/go-yogan-framework/testutil"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type inProcessTestApplication struct {
	core *application.Application
}

func (a *inProcessTestApplication) RunInProcess() error {
	return a.core.RunInProcess()
}

func (a *inProcessTestApplication) GetHTTPServer() interface{ GetEngine() *gin.Engine } {
	return a.core.GetHTTPServer()
}

func (a *inProcessTestApplication) GetDBManager() *database.Manager { return nil }
func (a *inProcessTestApplication) GetRedisManager() *redis.Manager { return nil }
func (a *inProcessTestApplication) Shutdown()                       { _ = a.core.Shutdown() }

type pingRoutes struct{}

func (pingRoutes) RegisterRoutes(engine *gin.Engine, app *application.Application) {
	engine.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})
}

func TestNewInProcessTestServerUsesRealRoutesWithoutBindingPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	tmpDir := t.TempDir()
	config := fmt.Sprintf(
		"api_server:\n  host: \"127.0.0.1\"\n  port: %d\n  mode: test\n",
		listener.Addr().(*net.TCPAddr).Port,
	)
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte(config), 0644))

	core := application.New(tmpDir, "TEST", nil)
	core.RegisterRoutes(pingRoutes{})
	app := &inProcessTestApplication{core: core}
	t.Cleanup(app.Shutdown)

	server, err := testutil.NewInProcessTestServer(app)
	require.NoError(t, err)
	require.NotNil(t, server)
	assert.Zero(t, core.GetHTTPServer().GetActualPort())

	request := httptest.NewRequest(http.MethodGet, "/ping", nil)
	response := httptest.NewRecorder()
	server.Engine.ServeHTTP(response, request)
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "pong", response.Body.String())
}
