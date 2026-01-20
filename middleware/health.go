package middleware

import (
	"net/http"

	"github.com/KOMKZ/go-yogan-framework/health"
	"github.com/gin-gonic/gin"
)

// HealthCheckHandler health check HTTP handler
// Provide a unified health check endpoint
type HealthCheckHandler struct {
	aggregator *health.Aggregator
}

// Create new health check handler
func NewHealthCheckHandler(aggregator *health.Aggregator) *HealthCheckHandler {
	return &HealthCheckHandler{
		aggregator: aggregator,
	}
}

// Handle health check request
// GET /health - Full health check
// GET /health/liveness - Liveness probe (K8s liveness probe)
// GET /health/readiness - Readiness probe (K8s readiness probe)
func (h *HealthCheckHandler) Handle() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Perform health check
		response := h.aggregator.Check(c.Request.Context())

		// Return HTTP status code based on overall state
		statusCode := http.StatusOK
		if response.Status == health.StatusUnhealthy {
			statusCode = http.StatusServiceUnavailable
		} else if response.Status == health.StatusDegraded {
			statusCode = http.StatusOK // Return 200 in degraded status but mark it in the response body
		}

		c.JSON(statusCode, response)
	}
}

// HandleLiveness K8s Liveness Probe
// A simple check to see if the application is alive (does not check dependencies)
func (h *HealthCheckHandler) HandleLiveness() gin.HandlerFunc {
	return func(c *gin.Context) {
		// The liveness probe only checks if the application itself is alive
		// Do not check for external dependencies (database, Redis, etc.)
		c.JSON(http.StatusOK, gin.H{
			"status": "alive",
		})
	}
}

// HandleReadiness K8s Readiness Probe
// Check if the application is ready (including all dependencies)
func (h *HealthCheckHandler) HandleReadiness() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Readiness probe checks if the application is ready to receive traffic
		// Include check for all dependencies
		response := h.aggregator.Check(c.Request.Context())

		// Only return 200 if fully healthy
		statusCode := http.StatusOK
		if response.Status != health.StatusHealthy {
			statusCode = http.StatusServiceUnavailable
		}

		c.JSON(statusCode, gin.H{
			"status": response.Status,
		})
	}
}

// RegisterHealthRoutes Register health check routes
// Convenient method, automatically registers all health check endpoints
func RegisterHealthRoutes(router gin.IRouter, aggregator *health.Aggregator) {
	if aggregator == nil {
		return
	}

	handler := NewHealthCheckHandler(aggregator)

	// Register routes
	router.GET("/health", handler.Handle())
	router.GET("/health/liveness", handler.HandleLiveness())
	router.GET("/health/readiness", handler.HandleReadiness())
}
