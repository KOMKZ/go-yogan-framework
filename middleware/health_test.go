package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/KOMKZ/go-yogan-framework/health"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// MockHealthChecker 模拟健康检查器
type MockHealthChecker struct {
	name string
	err  error // nil 表示健康，non-nil 表示不健康
}

func (m *MockHealthChecker) Name() string {
	return m.name
}

func (m *MockHealthChecker) Check(ctx context.Context) error {
	return m.err
}

func TestHealthCheckHandler_Handle_Healthy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// 创建健康聚合器
	aggregator := health.NewAggregator(5 * time.Second)
	aggregator.Register(&MockHealthChecker{name: "test", err: nil}) // nil = healthy

	handler := NewHealthCheckHandler(aggregator)
	router.GET("/health", handler.Handle())

	req := httptest.NewRequest("GET", "/health", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "healthy")
}

func TestHealthCheckHandler_Handle_Unhealthy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	aggregator := health.NewAggregator(5 * time.Second)
	aggregator.Register(&MockHealthChecker{name: "test", err: errors.New("unhealthy")})

	handler := NewHealthCheckHandler(aggregator)
	router.GET("/health", handler.Handle())

	req := httptest.NewRequest("GET", "/health", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusServiceUnavailable, resp.Code)
}

func TestHealthCheckHandler_HandleLiveness(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	aggregator := health.NewAggregator(5 * time.Second)
	handler := NewHealthCheckHandler(aggregator)
	router.GET("/health/liveness", handler.HandleLiveness())

	req := httptest.NewRequest("GET", "/health/liveness", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "alive")
}

func TestHealthCheckHandler_HandleReadiness_Ready(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	aggregator := health.NewAggregator(5 * time.Second)
	aggregator.Register(&MockHealthChecker{name: "test", err: nil})

	handler := NewHealthCheckHandler(aggregator)
	router.GET("/health/readiness", handler.HandleReadiness())

	req := httptest.NewRequest("GET", "/health/readiness", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHealthCheckHandler_HandleReadiness_NotReady(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	aggregator := health.NewAggregator(5 * time.Second)
	aggregator.Register(&MockHealthChecker{name: "test", err: errors.New("not ready")})

	handler := NewHealthCheckHandler(aggregator)
	router.GET("/health/readiness", handler.HandleReadiness())

	req := httptest.NewRequest("GET", "/health/readiness", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusServiceUnavailable, resp.Code)
}

func TestRegisterHealthRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	aggregator := health.NewAggregator(5 * time.Second)
	aggregator.Register(&MockHealthChecker{name: "test", err: nil})

	RegisterHealthRoutes(router, aggregator)

	// Test /health
	req := httptest.NewRequest("GET", "/health", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusOK, resp.Code)

	// Test /health/liveness
	req = httptest.NewRequest("GET", "/health/liveness", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusOK, resp.Code)

	// Test /health/readiness
	req = httptest.NewRequest("GET", "/health/readiness", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestRegisterHealthRoutes_NilAggregator(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// 传入 nil 应该安全返回
	RegisterHealthRoutes(router, nil)

	// 路由不应该存在
	req := httptest.NewRequest("GET", "/health", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusNotFound, resp.Code)
}
