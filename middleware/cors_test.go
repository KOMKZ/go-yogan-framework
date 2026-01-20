package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestDefaultCORSConfig(t *testing.T) {
	cfg := DefaultCORSConfig()

	assert.Equal(t, []string{"*"}, cfg.AllowOrigins)
	assert.Equal(t, []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}, cfg.AllowMethods)
	assert.Equal(t, []string{"Origin", "Content-Type", "Accept", "Authorization"}, cfg.AllowHeaders)
	assert.Equal(t, []string{}, cfg.ExposeHeaders)
	assert.False(t, cfg.AllowCredentials)
	assert.Equal(t, 43200, cfg.MaxAge)
}

func TestCORS_DefaultConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create test route
	router := gin.New()
	router.Use(CORS())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// test case
	tests := []struct {
		name           string
		method         string
		origin         string
		expectedStatus int
		checkHeaders   map[string]string
	}{
		{
			name:           "GET请求_允许所有源",
			method:         "GET",
			origin:         "https://example.com",
			expectedStatus: http.StatusOK,
			checkHeaders: map[string]string{
				"Access-Control-Allow-Origin":  "*",
				"Access-Control-Allow-Methods": "GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS",
				"Access-Control-Allow-Headers": "Origin, Content-Type, Accept, Authorization",
			},
		},
		{
			name:           "OPTIONS预检请求",
			method:         "OPTIONS",
			origin:         "https://example.com",
			expectedStatus: http.StatusNoContent,
			checkHeaders: map[string]string{
				"Access-Control-Allow-Origin":  "*",
				"Access-Control-Allow-Methods": "GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS",
				"Access-Control-Allow-Headers": "Origin, Content-Type, Accept, Authorization",
			},
		},
		{
			name:           "无Origin请求",
			method:         "GET",
			origin:         "",
			expectedStatus: http.StatusOK,
			checkHeaders:   map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request
			req := httptest.NewRequest(tt.method, "/test", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}

			// Log response
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Verify status code
			assert.Equal(t, tt.expectedStatus, w.Code)

			// Verify response headers
			for key, expectedValue := range tt.checkHeaders {
				assert.Equal(t, expectedValue, w.Header().Get(key), "响应头 %s 不匹配", key)
			}
		})
	}
}

func TestCORSWithConfig_CustomOrigins(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Custom configuration: allow only specific sources
	cfg := DefaultCORSConfig()
	cfg.AllowOrigins = []string{"https://example.com", "https://app.example.com"}

	// Create test route
	router := gin.New()
	router.Use(CORSWithConfig(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// test case
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
		{
			name:                "无Origin",
			origin:              "",
			expectedAllowOrigin: "",
			shouldHaveCORS:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}

			// Log response
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Validate status code
			assert.Equal(t, http.StatusOK, w.Code)

			// Verify CORS response headers
			if tt.shouldHaveCORS {
				assert.Equal(t, tt.expectedAllowOrigin, w.Header().Get("Access-Control-Allow-Origin"))
			} else {
				assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
			}
		})
	}
}

func TestCORSWithConfig_Credentials(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Custom configuration: Allow credentials
	cfg := DefaultCORSConfig()
	cfg.AllowOrigins = []string{"https://example.com"}
	cfg.AllowCredentials = true

	// Create test route
	router := gin.New()
	router.Use(CORSWithConfig(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// Create request
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://example.com")

	// Log response
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Validate
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "https://example.com", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
}

func TestCORSWithConfig_ExposeHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Custom configuration: expose response headers
	cfg := DefaultCORSConfig()
	cfg.ExposeHeaders = []string{"X-Request-ID", "X-Total-Count"}

	// Create test route
	router := gin.New()
	router.Use(CORSWithConfig(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// Create request
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://example.com")

	// Log response
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Validate
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "X-Request-ID, X-Total-Count", w.Header().Get("Access-Control-Expose-Headers"))
}

func TestCORSWithConfig_CustomMethods(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Custom configuration: allow specific methods only
	cfg := DefaultCORSConfig()
	cfg.AllowMethods = []string{"GET", "POST"}

	// Create test route
	router := gin.New()
	router.Use(CORSWithConfig(cfg))
	router.OPTIONS("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Create OPTIONS preflight request
	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "https://example.com")

	// Log response
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Validate
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "GET, POST", w.Header().Get("Access-Control-Allow-Methods"))
}

func TestCORSWithConfig_MaxAge(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Custom configuration: Set MaxAge
	cfg := DefaultCORSConfig()
	cfg.MaxAge = 7200 // 2 hours

	// Create test route
	router := gin.New()
	router.Use(CORSWithConfig(cfg))
	router.OPTIONS("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Create OPTIONS preflight request
	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "https://example.com")

	// Log response
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Validate
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "7200", w.Header().Get("Access-Control-Max-Age"))
}

