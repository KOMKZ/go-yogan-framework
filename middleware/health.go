package middleware

import (
	"net/http"

	"github.com/KOMKZ/go-yogan-framework/health"
	"github.com/gin-gonic/gin"
)

// HealthCheckHandler 健康检查 HTTP Handler
// 提供统一的健康检查端点
type HealthCheckHandler struct {
	healthComponent *health.Component
}

// NewHealthCheckHandler 创建健康检查 Handler
func NewHealthCheckHandler(healthComponent *health.Component) *HealthCheckHandler {
	return &HealthCheckHandler{
		healthComponent: healthComponent,
	}
}

// Handle 处理健康检查请求
// GET /health - 完整健康检查
// GET /health/liveness - 存活探针（K8s liveness probe）
// GET /health/readiness - 就绪探针（K8s readiness probe）
func (h *HealthCheckHandler) Handle() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 执行健康检查
		response := h.healthComponent.Check(c.Request.Context())

		// 根据整体状态返回 HTTP 状态码
		statusCode := http.StatusOK
		if response.Status == health.StatusUnhealthy {
			statusCode = http.StatusServiceUnavailable
		} else if response.Status == health.StatusDegraded {
			statusCode = http.StatusOK // 降级状态仍返回 200，但在响应体中标识
		}

		c.JSON(statusCode, response)
	}
}

// HandleLiveness K8s Liveness Probe
// 简单检查应用是否存活（不检查依赖项）
func (h *HealthCheckHandler) HandleLiveness() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Liveness 探针只检查应用本身是否存活
		// 不检查外部依赖（数据库、Redis 等）
		c.JSON(http.StatusOK, gin.H{
			"status": "alive",
		})
	}
}

// HandleReadiness K8s Readiness Probe
// 检查应用是否就绪（包括所有依赖项）
func (h *HealthCheckHandler) HandleReadiness() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Readiness 探针检查应用是否准备好接收流量
		// 包括检查所有依赖项
		response := h.healthComponent.Check(c.Request.Context())

		// 只有完全健康才返回 200
		statusCode := http.StatusOK
		if response.Status != health.StatusHealthy {
			statusCode = http.StatusServiceUnavailable
		}

		c.JSON(statusCode, gin.H{
			"status": response.Status,
		})
	}
}

// RegisterHealthRoutes 注册健康检查路由
// 便捷方法，自动注册所有健康检查端点
func RegisterHealthRoutes(router gin.IRouter, healthComponent *health.Component) {
	if healthComponent == nil || !healthComponent.IsEnabled() {
		return
	}

	handler := NewHealthCheckHandler(healthComponent)

	// 注册路由
	router.GET("/health", handler.Handle())
	router.GET("/health/liveness", handler.HandleLiveness())
	router.GET("/health/readiness", handler.HandleReadiness())
}
