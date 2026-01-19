package swagger

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestNewManager(t *testing.T) {
	cfg := Config{
		Enabled: true,
		UIPath:  "/swagger/*any",
	}
	info := DefaultSwaggerInfo()
	log := logger.GetLogger("test")

	mgr := NewManager(cfg, info, log)

	require.NotNil(t, mgr)
	assert.True(t, mgr.IsEnabled())
	assert.Equal(t, cfg.UIPath, mgr.GetConfig().UIPath)
	assert.Equal(t, info.Title, mgr.GetInfo().Title)
}

func TestManager_IsEnabled(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
	}{
		{"启用", true},
		{"禁用", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{Enabled: tt.enabled}
			mgr := NewManager(cfg, DefaultSwaggerInfo(), logger.GetLogger("test"))
			assert.Equal(t, tt.enabled, mgr.IsEnabled())
		})
	}
}

func TestManager_RegisterRoutes_Disabled(t *testing.T) {
	cfg := Config{Enabled: false}
	mgr := NewManager(cfg, DefaultSwaggerInfo(), logger.GetLogger("test"))

	engine := gin.New()
	mgr.RegisterRoutes(engine)

	// 禁用时不应注册路由
	routes := engine.Routes()
	assert.Empty(t, routes)
}

func TestManager_RegisterRoutes_Enabled(t *testing.T) {
	cfg := Config{
		Enabled:  true,
		UIPath:   "/swagger/*any",
		SpecPath: "/openapi.json",
	}
	mgr := NewManager(cfg, DefaultSwaggerInfo(), logger.GetLogger("test"))

	engine := gin.New()
	mgr.RegisterRoutes(engine)

	// 启用时应注册路由
	routes := engine.Routes()
	assert.Len(t, routes, 2) // UI + Spec

	// 验证路由路径
	paths := make([]string, len(routes))
	for i, r := range routes {
		paths[i] = r.Path
	}
	assert.Contains(t, paths, "/swagger/*any")
	assert.Contains(t, paths, "/openapi.json")
}

func TestManager_ServeSpec_NoDoc(t *testing.T) {
	cfg := Config{
		Enabled:  true,
		SpecPath: "/openapi.json",
	}
	mgr := NewManager(cfg, DefaultSwaggerInfo(), logger.GetLogger("test"))

	engine := gin.New()
	engine.GET("/openapi.json", mgr.serveSpec)

	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	// 未初始化 swag 时应返回错误
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestManager_Shutdown(t *testing.T) {
	mgr := NewManager(DefaultConfig(), DefaultSwaggerInfo(), logger.GetLogger("test"))
	err := mgr.Shutdown()
	assert.NoError(t, err)
}

func TestManager_GetConfig(t *testing.T) {
	cfg := Config{
		Enabled:  true,
		UIPath:   "/custom-swagger/*any",
		SpecPath: "/custom-spec.json",
	}
	mgr := NewManager(cfg, DefaultSwaggerInfo(), logger.GetLogger("test"))

	result := mgr.GetConfig()
	assert.Equal(t, cfg.UIPath, result.UIPath)
	assert.Equal(t, cfg.SpecPath, result.SpecPath)
}

func TestManager_GetInfo(t *testing.T) {
	info := SwaggerInfo{
		Title:       "Test API",
		Description: "Test Description",
		Version:     "2.0.0",
	}
	mgr := NewManager(DefaultConfig(), info, logger.GetLogger("test"))

	result := mgr.GetInfo()
	assert.Equal(t, info.Title, result.Title)
	assert.Equal(t, info.Description, result.Description)
	assert.Equal(t, info.Version, result.Version)
}
