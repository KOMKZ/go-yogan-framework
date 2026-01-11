package application

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TestHTTPServer_CORSIntegration 测试 CORS 中间件集成
func TestHTTPServer_CORSIntegration(t *testing.T) {
	tests := []struct {
		name          string
		corsEnabled   bool
		origin        string
		method        string
		expectedCORS  bool
		expectedOrigin string
	}{
		{
			name:          "CORS启用_允许所有源",
			corsEnabled:   true,
			origin:        "https://example.com",
			method:        "GET",
			expectedCORS:  true,
			expectedOrigin: "*",
		},
		{
			name:          "CORS启用_OPTIONS预检请求",
			corsEnabled:   true,
			origin:        "https://example.com",
			method:        "OPTIONS",
			expectedCORS:  true,
			expectedOrigin: "*",
		},
		{
			name:        "CORS禁用",
			corsEnabled: false,
			origin:      "https://example.com",
			method:      "GET",
			expectedCORS: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建配置
			apiCfg := ApiServerConfig{
				Host:         "0.0.0.0",
				Port:         8080,
				Mode:         "test",
				ReadTimeout:  60,
				WriteTimeout: 60,
			}

			middlewareCfg := &MiddlewareConfig{}
			if tt.corsEnabled {
				middlewareCfg.CORS = &CORSConfig{
					Enable:       true,
					AllowOrigins: []string{"*"},
				}
			}

			// 应用默认值
			middlewareCfg.ApplyDefaults()

			// 创建 HTTP Server（测试环境默认不记录错误日志）
			server := NewHTTPServer(apiCfg, middlewareCfg, nil, nil)

			// 注册测试路由
			server.GetEngine().GET("/test", func(c *gin.Context) {
				c.JSON(http.StatusOK, map[string]string{"message": "success"})
			})

			// 创建请求
			req := httptest.NewRequest(tt.method, "/test", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}

			// 记录响应
			w := httptest.NewRecorder()
			server.GetEngine().ServeHTTP(w, req)

			// 验证 CORS 响应头
			if tt.expectedCORS {
				assert.Equal(t, tt.expectedOrigin, w.Header().Get("Access-Control-Allow-Origin"))
				assert.NotEmpty(t, w.Header().Get("Access-Control-Allow-Methods"))
				assert.NotEmpty(t, w.Header().Get("Access-Control-Allow-Headers"))
				
				// OPTIONS 预检请求应返回 204
				if tt.method == "OPTIONS" {
					assert.Equal(t, http.StatusNoContent, w.Code)
				}
			} else {
				assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
			}
		})
	}
}

// TestHTTPServer_CORSWithCustomOrigins 测试 CORS 自定义源白名单
func TestHTTPServer_CORSWithCustomOrigins(t *testing.T) {
	// 创建配置
	apiCfg := ApiServerConfig{
		Host:         "0.0.0.0",
		Port:         8080,
		Mode:         "test",
		ReadTimeout:  60,
		WriteTimeout: 60,
	}

	middlewareCfg := &MiddlewareConfig{
		CORS: &CORSConfig{
			Enable: true,
			AllowOrigins: []string{
				"https://example.com",
				"https://app.example.com",
			},
			AllowCredentials: true,
		},
	}

	// 应用默认值
	middlewareCfg.ApplyDefaults()

	// 创建 HTTP Server（测试环境默认不记录错误日志）
	server := NewHTTPServer(apiCfg, middlewareCfg, nil)

	// 注册测试路由
	server.GetEngine().GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "success"})
	})

	tests := []struct {
		name                string
		origin              string
		expectedAllowOrigin string
		shouldHaveCORS      bool
	}{
		{
			name:                "允许的源_1",
			origin:              "https://example.com",
			expectedAllowOrigin: "https://example.com",
			shouldHaveCORS:      true,
		},
		{
			name:                "允许的源_2",
			origin:              "https://app.example.com",
			expectedAllowOrigin: "https://app.example.com",
			shouldHaveCORS:      true,
		},
		{
			name:                "不允许的源",
			origin:              "https://evil.com",
			expectedAllowOrigin: "",
			shouldHaveCORS:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建请求
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("Origin", tt.origin)

			// 记录响应
			w := httptest.NewRecorder()
			server.GetEngine().ServeHTTP(w, req)

			// 验证
			if tt.shouldHaveCORS {
				assert.Equal(t, tt.expectedAllowOrigin, w.Header().Get("Access-Control-Allow-Origin"))
				assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
			} else {
				assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
			}
		})
	}
}

