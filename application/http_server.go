package application

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/KOMKZ/go-yogan-framework/health"
	"github.com/KOMKZ/go-yogan-framework/httpx"
	"github.com/KOMKZ/go-yogan-framework/jwt"
	"github.com/KOMKZ/go-yogan-framework/limiter"
	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/KOMKZ/go-yogan-framework/middleware"
	"github.com/KOMKZ/go-yogan-framework/telemetry"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.uber.org/zap"
)

// HTTPServer wraps an HTTP server (supports Gin)
type HTTPServer struct {
	engine     *gin.Engine
	httpServer *http.Server
	port       int // Configured port (0 = auto-assign)
	actualPort int // Actual listening port, resolved at bind time
	mode       string
}

// NewHTTPServer creates an HTTP server (uniform logging solution)
func NewHTTPServer(cfg ApiServerConfig, middlewareCfg *MiddlewareConfig, httpxCfg *httpx.ErrorLoggingConfig, limiterManager *limiter.Manager) *HTTPServer {
	return newServer(cfg, middlewareCfg, httpxCfg, limiterManager, nil, nil)
}

// newServer builds the gin engine with the shared middleware assembly.
// 🎯 Single source of truth for middleware order and defaults:
// NewHTTPServer and NewHTTPServerWithTelemetry delegate here (they used to
// duplicate ~100 lines and drifted apart). telemetryMgr nil skips the
// OpenTelemetry span middleware.
func newServer(
	cfg ApiServerConfig,
	middlewareCfg *MiddlewareConfig,
	httpxCfg *httpx.ErrorLoggingConfig,
	limiterManager *limiter.Manager,
	telemetryMgr *telemetry.Manager,
	tokenManager jwt.TokenManager,
) *HTTPServer {
	// ====================================
	// 1. Take over Gin core log output (avoid the built-in Logger/Recovery)
	// ====================================
	gin.DefaultWriter = logger.NewGinLogWriter("yogan")
	gin.DefaultErrorWriter = logger.NewGinLogWriter("yogan")

	// ====================================
	// 2. Set Gin mode (debug: route logs; release: production)
	// ====================================
	gin.SetMode(cfg.Mode)

	// ====================================
	// 3. Create Gin engine
	// ====================================
	engine := gin.New()

	// Enable 405 method not allowed response (default is 404)
	engine.HandleMethodNotAllowed = true

	// ====================================
	// 4. Middleware chain (order matters)
	// ====================================

	// CORS: must be at the top so pre-flight requests are handled correctly
	if middlewareCfg != nil && middlewareCfg.CORS != nil && middlewareCfg.CORS.Enable {
		corsCfg := middleware.CORSConfig{
			AllowOrigins:     middlewareCfg.CORS.AllowOrigins,
			AllowMethods:     middlewareCfg.CORS.AllowMethods,
			AllowHeaders:     middlewareCfg.CORS.AllowHeaders,
			ExposeHeaders:    middlewareCfg.CORS.ExposeHeaders,
			AllowCredentials: middlewareCfg.CORS.AllowCredentials,
			MaxAge:           middlewareCfg.CORS.MaxAge,
		}
		engine.Use(middleware.CORSWithConfig(corsCfg))
	}

	// OpenTelemetry span middleware (must be before TraceID so the span
	// exists when TraceID extracts the ID)
	if telemetryMgr != nil && telemetryMgr.IsEnabled() {
		serviceName := telemetryMgr.GetConfig().ServiceName
		if serviceName == "" {
			serviceName = "http-service"
		}
		engine.Use(otelgin.Middleware(serviceName))
		logger.Info("yogan", "✅ OpenTelemetry Trace middleware registered",
			zap.String("service_name", serviceName))
	}

	// TraceID: generates/extracts the TraceID for each request
	// (must be before the logging middleware)
	if middlewareCfg != nil && middlewareCfg.TraceID != nil && middlewareCfg.TraceID.Enable {
		traceCfg := middleware.TraceConfig{
			TraceIDKey:           middlewareCfg.TraceID.TraceIDKey,
			TraceIDHeader:        middlewareCfg.TraceID.TraceIDHeader,
			EnableResponseHeader: middlewareCfg.TraceID.EnableResponseHeader,
		}
		engine.Use(middleware.TraceID(traceCfg))
	}

	// Rate limiting: before the logging middleware so rate-limit events
	// are also recorded
	if limiterManager != nil && limiterManager.IsEnabled() {
		limiterCfg := limiterManager.GetConfig()
		rateLimiterCfg := middleware.DefaultRateLimiterConfig(limiterManager)
		rateLimiterCfg.Rules = limiterCfg.Rules
		rateLimiterCfg.TokenManager = tokenManager

		// skip rate-limited paths
		if len(limiterCfg.SkipPaths) > 0 {
			rateLimiterCfg.SkipPaths = limiterCfg.SkipPaths
		}

		// Choose key function based on configuration
		switch limiterCfg.KeyFunc {
		case "ip":
			rateLimiterCfg.KeyFunc = middleware.RateLimiterKeyByIP
		case "user":
			rateLimiterCfg.KeyFunc = middleware.RateLimiterKeyByUser("user_id")
		case "user_path":
			rateLimiterCfg.KeyFunc = middleware.RateLimiterKeyByUserAndPath("user_id")
		case "path_ip":
			rateLimiterCfg.KeyFunc = middleware.RateLimiterKeyByPathAndIP
		case "api_key":
			rateLimiterCfg.KeyFunc = middleware.RateLimiterKeyByAPIKey("X-API-Key")
		case "path", "":
			// Default: METHOD:PATH (already set in DefaultRateLimiterConfig)
		default:
			logger.Warn("yogan", "Unknown KeyFunc config, using default",
				zap.String("key_func", limiterCfg.KeyFunc))
		}

		engine.Use(middleware.RateLimiterWithConfig(rateLimiterCfg))
		logger.Debug("yogan", "✅ Rate limiter middleware globally enabled",
			zap.String("key_func", limiterCfg.KeyFunc))
	}

	// HTTP request logging (automatically associates TraceID)
	if middlewareCfg != nil && middlewareCfg.RequestLog != nil && middlewareCfg.RequestLog.Enable {
		requestLogCfg := middleware.RequestLogConfig{
			SkipPaths:   middlewareCfg.RequestLog.SkipPaths,
			EnableBody:  middlewareCfg.RequestLog.EnableBody,
			MaxBodySize: middlewareCfg.RequestLog.MaxBodySize,
		}
		engine.Use(middleware.RequestLogWithConfig(requestLogCfg))
	}

	// HTTP error logging: decides based on configuration whether to log
	// business error logs (default is not to log)
	if httpxCfg != nil && httpxCfg.Enable {
		engine.Use(httpx.ErrorLoggingMiddleware(*httpxCfg))
	}

	// Panic recovery (always enabled)
	engine.Use(middleware.Recovery())

	// ====================================
	// Unified response handling for 404/405
	// ====================================
	engine.NoRoute(httpx.NoRouteHandler())
	engine.NoMethod(httpx.NoMethodHandler())

	return &HTTPServer{
		engine: engine,
		port:   cfg.Port,
		mode:   cfg.Mode,
	}
}

// GetEngine retrieves the Gin engine (for business layer route registration)
func (s *HTTPServer) GetEngine() *gin.Engine {
	return s.engine
}

// Start non-blocking HTTP Server (will wait for confirmation of successful startup)
// 🎯 The listener is bound first so the actual port is known immediately
// (port 0 = auto-assigned, retrievable via GetActualPort) and there is no
// TOCTOU gap between an availability pre-check and the real bind.
func (s *HTTPServer) Start() error {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return fmt.Errorf("Port %d is not available: %w", s.port, err)
	}
	s.actualPort = ln.Addr().(*net.TCPAddr).Port

	s.httpServer = &http.Server{
		Addr:    ln.Addr().String(),
		Handler: s.engine,
	}

	// 1. Use channel to wait for startup result
	errChan := make(chan error, 1)

	go func() {
		logger.Debug("yogan", "🚀 HTTP server starting",
			zap.Int("port", s.actualPort),
			zap.String("mode", s.mode))

		if err := s.httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// 2. Briefly wait to confirm successful startup (50ms is sufficient to detect serving errors)
	select {
	case err := <-errChan:
		logger.Error("yogan", "❌ HTTP server start failed", zap.Error(err))
		return fmt.Errorf("HTTP service startup failed: %w", err)
	case <-time.After(50 * time.Millisecond):
		// startup successful
		logger.Debug("yogan", "✅ HTTP server started successfully",
			zap.Int("port", s.actualPort))
		return nil
	}
}

// GetActualPort returns the actual listening port, resolved at bind time
// (0 until Start succeeds). For port 0 configuration this is the only way
// to obtain the real port.
func (s *HTTPServer) GetActualPort() int {
	return s.actualPort
}

// Shut down HTTP Server gracefully
func (s *HTTPServer) Shutdown(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}

	logger.Debug("yogan", "Shutting down HTTP server...")

	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("HTTP Server shutdown failed: %w", err)
	}

	logger.Debug("yogan", "✅ HTTP server closed")
	return nil
}

// ShutdownWithTimeout graceful shutdown with timeout
func (s *HTTPServer) ShutdownWithTimeout(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return s.Shutdown(ctx)
}

// Create an HTTP server with OpenTelemetry and health check support
func NewHTTPServerWithTelemetryAndHealth(
	cfg ApiServerConfig,
	middlewareCfg *MiddlewareConfig,
	httpxCfg *httpx.ErrorLoggingConfig,
	limiterManager *limiter.Manager,
	telemetryMgr *telemetry.Manager,
	healthAgg *health.Aggregator, // Use specific types, avoid interface{}
) *HTTPServer {
	return NewHTTPServerWithTelemetryAndHealthWithJWT(cfg, middlewareCfg, httpxCfg, limiterManager, telemetryMgr, healthAgg, nil)
}

// Create an HTTP server with OpenTelemetry, health check, and JWT-aware middleware support.
func NewHTTPServerWithTelemetryAndHealthWithJWT(
	cfg ApiServerConfig,
	middlewareCfg *MiddlewareConfig,
	httpxCfg *httpx.ErrorLoggingConfig,
	limiterManager *limiter.Manager,
	telemetryMgr *telemetry.Manager,
	healthAgg *health.Aggregator, // Use specific types, avoid interface{}
	tokenManager jwt.TokenManager,
) *HTTPServer {
	server := NewHTTPServerWithTelemetryWithJWT(cfg, middlewareCfg, httpxCfg, limiterManager, telemetryMgr, tokenManager)

	// Register health check route
	middleware.RegisterHealthRoutes(server.engine, healthAgg)

	return server
}

// Create an HTTP server with OpenTelemetry support
// 🎯 Delegates to newServer with the telemetry manager; the shared assembly
// lives in exactly one place (see newServer).
func NewHTTPServerWithTelemetry(
	cfg ApiServerConfig,
	middlewareCfg *MiddlewareConfig,
	httpxCfg *httpx.ErrorLoggingConfig,
	limiterManager *limiter.Manager,
	telemetryMgr *telemetry.Manager,
) *HTTPServer {
	return NewHTTPServerWithTelemetryWithJWT(cfg, middlewareCfg, httpxCfg, limiterManager, telemetryMgr, nil)
}

// Create an HTTP server with OpenTelemetry and JWT-aware middleware support.
func NewHTTPServerWithTelemetryWithJWT(
	cfg ApiServerConfig,
	middlewareCfg *MiddlewareConfig,
	httpxCfg *httpx.ErrorLoggingConfig,
	limiterManager *limiter.Manager,
	telemetryMgr *telemetry.Manager,
	tokenManager jwt.TokenManager,
) *HTTPServer {
	return newServer(cfg, middlewareCfg, httpxCfg, limiterManager, telemetryMgr, tokenManager)
}
