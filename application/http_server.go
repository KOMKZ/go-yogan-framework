package application

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/KOMKZ/go-yogan-framework/health"
	"github.com/KOMKZ/go-yogan-framework/httpx"
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
	port       int
	mode       string
}

// NewHTTPServer creates an HTTP server (uniform logging solution)
func NewHTTPServer(cfg ApiServerConfig, middlewareCfg *MiddlewareConfig, httpxCfg *httpx.ErrorLoggingConfig, limiterManager *limiter.Manager) *HTTPServer {
	// ====================================
	// Take over Gin core log output
	// ====================================
	// Redirect Gin's routing registration logs to a custom Logger
	gin.DefaultWriter = logger.NewGinLogWriter("yogan")
	// Redirect Gin's error logs to a custom logger
	gin.DefaultErrorWriter = logger.NewGinLogWriter("yogan")

	// ====================================
	// 2. Set Gin mode
	// ====================================
	// debug: output detailed route registration logs
	// release: disable route registration log (recommended for production environment)
	gin.SetMode(cfg.Mode)

	// ====================================
	// 3. Create Gin engine
	// ====================================
	// Use gin.New() instead of gin.Default()
	// Avoid using the built-in Logger and Recovery middleware, use a custom version
	engine := gin.New()

	// Enable 405 method not allowed response (default is 404)
	engine.HandleMethodNotAllowed = true

	// ====================================
	// 4. Register custom middleware (based on configuration, note the order)
	// ====================================

	// CORS middleware: Handle cross-origin requests (must be at the top to ensure pre-flight requests are correctly responded to)
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

	// TraceID middleware: Generates/extracts TraceID for each request (must be before log middleware)
	if middlewareCfg != nil && middlewareCfg.TraceID != nil && middlewareCfg.TraceID.Enable {
		traceCfg := middleware.TraceConfig{
			TraceIDKey:           middlewareCfg.TraceID.TraceIDKey,
			TraceIDHeader:        middlewareCfg.TraceID.TraceIDHeader,
			EnableResponseHeader: middlewareCfg.TraceID.EnableResponseHeader,
		}
		engine.Use(middleware.TraceID(traceCfg))
	}

	// Rate limiting middleware: globally applied rate limiting (applied before the logging middleware so that rate-limiting events are also recorded)
	if limiterManager != nil && limiterManager.IsEnabled() {
		limiterCfg := limiterManager.GetConfig()
		rateLimiterCfg := middleware.DefaultRateLimiterConfig(limiterManager)

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

	// HTTP request logging middleware: log all HTTP requests to the gin-http module (automatically associate TraceID)
	if middlewareCfg != nil && middlewareCfg.RequestLog != nil && middlewareCfg.RequestLog.Enable {
		requestLogCfg := middleware.RequestLogConfig{
			SkipPaths:   middlewareCfg.RequestLog.SkipPaths,
			EnableBody:  middlewareCfg.RequestLog.EnableBody,
			MaxBodySize: middlewareCfg.RequestLog.MaxBodySize,
		}
		engine.Use(middleware.RequestLogWithConfig(requestLogCfg))
	}

	// HTTP error logging middleware: decides based on configuration whether to log business error logs (default is not to log)
	if httpxCfg != nil && httpxCfg.Enable {
		engine.Use(httpx.ErrorLoggingMiddleware(*httpxCfg))
	}

	// Panic recovery middleware: captures panics and logs to the gin-error module (always enabled)
	engine.Use(middleware.Recovery())

	// ====================================
	// Register unified response handling for 404/405 errors
	// ====================================
	engine.NoRoute(httpx.NoRouteHandler())
	engine.NoMethod(httpx.NoMethodHandler())

	return &HTTPServer{
		engine: engine,
		port:   cfg.Port,
		mode:   cfg.Mode,
	}
}

// GetEngine获取Gin引擎（用于业务层注册路由）
func (s *HTTPServer) GetEngine() *gin.Engine {
	return s.engine
}

// Start non-blocking HTTP Server (will wait for confirmation of successful startup)
func (s *HTTPServer) Start() error {
	addr := fmt.Sprintf(":%d", s.port)

	// 1. Pre-check port availability
	if err := s.checkPortAvailable(); err != nil {
		return fmt.Errorf("端口 %d 不可用: %w", s.port, err)
	}

	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: s.engine,
	}

	// 2. Use channel to wait for startup result
	errChan := make(chan error, 1)

	go func() {
		logger.Debug("yogan", "🚀 HTTP server starting",
			zap.Int("port", s.port),
			zap.String("mode", s.mode))

		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// 3. Briefly wait to confirm successful startup (50ms is sufficient to detect port binding errors)
	select {
	case err := <-errChan:
		logger.Error("yogan", "❌ HTTP server start failed", zap.Error(err))
		return fmt.Errorf("HTTP 服务启动失败: %w", err)
	case <-time.After(50 * time.Millisecond):
		// startup successful
		logger.Debug("yogan", "✅ HTTP server started successfully",
			zap.Int("port", s.port))
		return nil
	}
}

// checkPortAvailable Check if the port is available
func (s *HTTPServer) checkPortAvailable() error {
	addr := fmt.Sprintf(":%d", s.port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	ln.Close()
	return nil
}

// Shut down HTTP Server gracefully
func (s *HTTPServer) Shutdown(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}

	logger.Debug("yogan", "Shutting down HTTP server...")

	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("HTTP Server 关闭失败: %w", err)
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
	server := NewHTTPServerWithTelemetry(cfg, middlewareCfg, httpxCfg, limiterManager, telemetryMgr)

	// Register health check route
	middleware.RegisterHealthRoutes(server.engine, healthAgg)

	return server
}

// Create an HTTP server with OpenTelemetry support
func NewHTTPServerWithTelemetry(
	cfg ApiServerConfig,
	middlewareCfg *MiddlewareConfig,
	httpxCfg *httpx.ErrorLoggingConfig,
	limiterManager *limiter.Manager,
	telemetryMgr *telemetry.Manager,
) *HTTPServer {
	// ====================================
	// Take over Gin core log output
	// ====================================
	gin.DefaultWriter = logger.NewGinLogWriter("yogan")
	gin.DefaultErrorWriter = logger.NewGinLogWriter("yogan")

	// ====================================
	// 2. Set Gin mode
	// ====================================
	gin.SetMode(cfg.Mode)

	// ====================================
	// 3. Create Gin engine
	// ====================================
	engine := gin.New()

	// Enable 405 method not allowed response (default is 404)
	engine.HandleMethodNotAllowed = true

	// ====================================
	// 4. Register custom middleware (note the order)
	// ====================================

	// CORS middleware: Handle cross-domain requests (must be at the very top)
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

	// 🎯 OpenTelemetry Trace middleware: Create a Span (must be before TraceID)
	if telemetryMgr != nil && telemetryMgr.IsEnabled() {
		serviceName := telemetryMgr.GetConfig().ServiceName
		if serviceName == "" {
			serviceName = "http-service"
		}
		engine.Use(otelgin.Middleware(serviceName))
		logger.Info("yogan", "✅ OpenTelemetry Trace middleware registered",
			zap.String("service_name", serviceName))
	}

	// 🎯 HTTP Metrics middleware: collect HTTP request metrics (independent of Trace)
	if telemetryMgr != nil {
		metricsMgr := telemetryMgr.GetMetricsManager()
		if metricsMgr != nil {
			// Metrics have been started in the Manager
			logger.Info("yogan", "✅ HTTP Metrics middleware available via Telemetry Manager")
		}
	}

	// TraceID middleware: Extract TraceID from Span or Header (after otelgin)
	if middlewareCfg != nil && middlewareCfg.TraceID != nil && middlewareCfg.TraceID.Enable {
		traceCfg := middleware.TraceConfig{
			TraceIDKey:           middlewareCfg.TraceID.TraceIDKey,
			TraceIDHeader:        middlewareCfg.TraceID.TraceIDHeader,
			EnableResponseHeader: middlewareCfg.TraceID.EnableResponseHeader,
		}
		engine.Use(middleware.TraceID(traceCfg))
	}

	// Rate limiting middleware: global rate limiting applied (before the logging middleware so that rate limiting events are also recorded)
	if limiterManager != nil && limiterManager.IsEnabled() {
		limiterCfg := limiterManager.GetConfig()
		rateLimiterCfg := middleware.DefaultRateLimiterConfig(limiterManager)

		// Bypass rate-limited paths
		if len(limiterCfg.SkipPaths) > 0 {
			rateLimiterCfg.SkipPaths = limiterCfg.SkipPaths
		}

		// Choose key function based on configuration
		switch limiterCfg.KeyFunc {
		case "ip":
			rateLimiterCfg.KeyFunc = middleware.RateLimiterKeyByIP
		case "user":
			rateLimiterCfg.KeyFunc = middleware.RateLimiterKeyByUser("user_id")
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

	// HTTP request logging middleware
	if middlewareCfg != nil && middlewareCfg.RequestLog != nil && middlewareCfg.RequestLog.Enable {
		requestLogCfg := middleware.RequestLogConfig{
			SkipPaths:   middlewareCfg.RequestLog.SkipPaths,
			EnableBody:  middlewareCfg.RequestLog.EnableBody,
			MaxBodySize: middlewareCfg.RequestLog.MaxBodySize,
		}
		engine.Use(middleware.RequestLogWithConfig(requestLogCfg))
	}

	// HTTP error logging middleware
	if httpxCfg != nil && httpxCfg.Enable {
		engine.Use(httpx.ErrorLoggingMiddleware(*httpxCfg))
	}

	// Enable middleware for panic recovery (always enabled)
	engine.Use(middleware.Recovery())

	// ====================================
	// Register uniform response handling for 404/405
	// ====================================
	engine.NoRoute(httpx.NoRouteHandler())
	engine.NoMethod(httpx.NoMethodHandler())

	return &HTTPServer{
		engine: engine,
		port:   cfg.Port,
		mode:   cfg.Mode,
	}
}
