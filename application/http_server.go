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

// HTTPServer HTTP Server 封装（支持 Gin）
type HTTPServer struct {
	engine     *gin.Engine
	httpServer *http.Server
	port       int
	mode       string
}

// NewHTTPServer 创建 HTTP Server（完整日志统一方案）
func NewHTTPServer(cfg ApiServerConfig, middlewareCfg *MiddlewareConfig, httpxCfg *httpx.ErrorLoggingConfig, limiterManager *limiter.Manager) *HTTPServer {
	// ====================================
	// 1. 接管 Gin 内核日志输出
	// ====================================
	// 将 Gin 的路由注册日志重定向到自定义 Logger
	gin.DefaultWriter = logger.NewGinLogWriter("yogan")
	// 将 Gin 的错误日志重定向到自定义 Logger
	gin.DefaultErrorWriter = logger.NewGinLogWriter("yogan")

	// ====================================
	// 2. 设置 Gin 模式
	// ====================================
	// debug: 输出详细的路由注册日志
	// release: 关闭路由注册日志（生产环境推荐）
	gin.SetMode(cfg.Mode)

	// ====================================
	// 3. 创建 Gin 引擎
	// ====================================
	// 使用 gin.New() 而非 gin.Default()
	// 避免自带的 Logger 和 Recovery 中间件，使用自定义版本
	engine := gin.New()

	// 启用 405 方法不允许响应（默认是 404）
	engine.HandleMethodNotAllowed = true

	// ====================================
	// 4. 注册自定义中间件（根据配置，注意顺序）
	// ====================================

	// CORS 中间件：处理跨域请求（必须在最前面，确保预检请求能正确响应）
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

	// TraceID 中间件：为每个请求生成/提取 TraceID（必须在日志中间件之前）
	if middlewareCfg != nil && middlewareCfg.TraceID != nil && middlewareCfg.TraceID.Enable {
		traceCfg := middleware.TraceConfig{
			TraceIDKey:           middlewareCfg.TraceID.TraceIDKey,
			TraceIDHeader:        middlewareCfg.TraceID.TraceIDHeader,
			EnableResponseHeader: middlewareCfg.TraceID.EnableResponseHeader,
		}
		engine.Use(middleware.TraceID(traceCfg))
	}

	// 限流中间件：全局应用限流（在日志中间件之前，这样限流事件也会被记录）
	if limiterManager != nil && limiterManager.IsEnabled() {
		limiterCfg := limiterManager.GetConfig()
		rateLimiterCfg := middleware.DefaultRateLimiterConfig(limiterManager)

		// 跳过限流的路径
		if len(limiterCfg.SkipPaths) > 0 {
			rateLimiterCfg.SkipPaths = limiterCfg.SkipPaths
		}

		// 根据配置选择键函数
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
			// 默认：METHOD:PATH（已在 DefaultRateLimiterConfig 中设置）
		default:
			logger.Warn("yogan", "Unknown KeyFunc config, using default",
				zap.String("key_func", limiterCfg.KeyFunc))
		}

		engine.Use(middleware.RateLimiterWithConfig(rateLimiterCfg))
		logger.Debug("yogan", "✅ Rate limiter middleware globally enabled",
			zap.String("key_func", limiterCfg.KeyFunc))
	}

	// HTTP 请求日志中间件：记录所有 HTTP 请求到 gin-http 模块（自动关联 TraceID）
	if middlewareCfg != nil && middlewareCfg.RequestLog != nil && middlewareCfg.RequestLog.Enable {
		requestLogCfg := middleware.RequestLogConfig{
			SkipPaths:   middlewareCfg.RequestLog.SkipPaths,
			EnableBody:  middlewareCfg.RequestLog.EnableBody,
			MaxBodySize: middlewareCfg.RequestLog.MaxBodySize,
		}
		engine.Use(middleware.RequestLogWithConfig(requestLogCfg))
	}

	// HTTP 错误日志中间件：根据配置决定是否记录业务错误日志（默认不记录）
	if httpxCfg != nil && httpxCfg.Enable {
		engine.Use(httpx.ErrorLoggingMiddleware(*httpxCfg))
	}

	// Panic 恢复中间件：捕获 panic 并记录到 gin-error 模块（总是启用）
	engine.Use(middleware.Recovery())

	// ====================================
	// 5. 注册 404/405 统一响应处理
	// ====================================
	engine.NoRoute(httpx.NoRouteHandler())
	engine.NoMethod(httpx.NoMethodHandler())

	return &HTTPServer{
		engine: engine,
		port:   cfg.Port,
		mode:   cfg.Mode,
	}
}

// GetEngine 获取 Gin 引擎（供业务层注册路由）
func (s *HTTPServer) GetEngine() *gin.Engine {
	return s.engine
}

// Start 启动 HTTP Server（非阻塞，但会等待确认启动成功）
func (s *HTTPServer) Start() error {
	addr := fmt.Sprintf(":%d", s.port)

	// 1. 预检测端口可用性
	if err := s.checkPortAvailable(); err != nil {
		return fmt.Errorf("端口 %d 不可用: %w", s.port, err)
	}

	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: s.engine,
	}

	// 2. 使用 channel 等待启动结果
	errChan := make(chan error, 1)

	go func() {
		logger.Debug("yogan", "🚀 HTTP server starting",
			zap.Int("port", s.port),
			zap.String("mode", s.mode))

		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// 3. 短暂等待确认启动成功（50ms 足够检测端口绑定错误）
	select {
	case err := <-errChan:
		logger.Error("yogan", "❌ HTTP server start failed", zap.Error(err))
		return fmt.Errorf("HTTP 服务启动失败: %w", err)
	case <-time.After(50 * time.Millisecond):
		// 启动成功
		logger.Debug("yogan", "✅ HTTP server started successfully",
			zap.Int("port", s.port))
		return nil
	}
}

// checkPortAvailable 检测端口是否可用
func (s *HTTPServer) checkPortAvailable() error {
	addr := fmt.Sprintf(":%d", s.port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	ln.Close()
	return nil
}

// Shutdown 优雅关闭 HTTP Server
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

// ShutdownWithTimeout 带超时的优雅关闭
func (s *HTTPServer) ShutdownWithTimeout(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return s.Shutdown(ctx)
}

// NewHTTPServerWithTelemetryAndHealth 创建带 OpenTelemetry 和健康检查支持的 HTTP Server
func NewHTTPServerWithTelemetryAndHealth(
	cfg ApiServerConfig,
	middlewareCfg *MiddlewareConfig,
	httpxCfg *httpx.ErrorLoggingConfig,
	limiterManager *limiter.Manager,
	telemetryComp *telemetry.Component,
	healthComp *health.Component, // 使用具体类型，避免 interface{}
) *HTTPServer {
	server := NewHTTPServerWithTelemetry(cfg, middlewareCfg, httpxCfg, limiterManager, telemetryComp)

	// 注册健康检查路由
	middleware.RegisterHealthRoutes(server.engine, healthComp)

	return server
}

// NewHTTPServerWithTelemetry 创建带 OpenTelemetry 支持的 HTTP Server
func NewHTTPServerWithTelemetry(
	cfg ApiServerConfig,
	middlewareCfg *MiddlewareConfig,
	httpxCfg *httpx.ErrorLoggingConfig,
	limiterManager *limiter.Manager,
	telemetryComp *telemetry.Component,
) *HTTPServer {
	// ====================================
	// 1. 接管 Gin 内核日志输出
	// ====================================
	gin.DefaultWriter = logger.NewGinLogWriter("yogan")
	gin.DefaultErrorWriter = logger.NewGinLogWriter("yogan")

	// ====================================
	// 2. 设置 Gin 模式
	// ====================================
	gin.SetMode(cfg.Mode)

	// ====================================
	// 3. 创建 Gin 引擎
	// ====================================
	engine := gin.New()

	// 启用 405 方法不允许响应（默认是 404）
	engine.HandleMethodNotAllowed = true

	// ====================================
	// 4. 注册自定义中间件（注意顺序）
	// ====================================

	// CORS 中间件：处理跨域请求（必须在最前面）
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

	// 🎯 OpenTelemetry Trace 中间件：创建 Span（必须在 TraceID 之前）
	if telemetryComp != nil && telemetryComp.IsEnabled() {
		serviceName := telemetryComp.GetConfig().ServiceName
		if serviceName == "" {
			serviceName = "http-service"
		}
		engine.Use(otelgin.Middleware(serviceName, otelgin.WithTracerProvider(telemetryComp.GetTracerProvider())))
		logger.Info("yogan", "✅ OpenTelemetry Trace middleware registered",
			zap.String("service_name", serviceName))
	}

	// 🎯 HTTP Metrics 中间件：收集 HTTP 请求指标（独立于 Trace）
	if telemetryComp != nil {
		metricsManager := telemetryComp.GetMetricsManager()
		metricsRegistry := telemetryComp.GetMetricsRegistry()
		if metricsManager != nil && metricsManager.IsHTTPMetricsEnabled() {
			httpMetrics := middleware.NewHTTPMetrics(middleware.HTTPMetricsConfig{
				Enabled:            metricsManager.GetConfig().HTTP.Enabled,
				RecordRequestSize:  metricsManager.GetConfig().HTTP.RecordRequestSize,
				RecordResponseSize: metricsManager.GetConfig().HTTP.RecordResponseSize,
			})
			// Register with MetricsRegistry if available
			if metricsRegistry != nil {
				if err := metricsRegistry.Register(httpMetrics); err != nil {
					logger.Warn("yogan", "Failed to register HTTP Metrics", zap.Error(err))
				}
			}
			engine.Use(httpMetrics.Handler())
			logger.Info("yogan", "✅ HTTP Metrics middleware registered")
		}
	}

	// TraceID 中间件：从 Span 或 Header 提取 TraceID（在 otelgin 之后）
	if middlewareCfg != nil && middlewareCfg.TraceID != nil && middlewareCfg.TraceID.Enable {
		traceCfg := middleware.TraceConfig{
			TraceIDKey:           middlewareCfg.TraceID.TraceIDKey,
			TraceIDHeader:        middlewareCfg.TraceID.TraceIDHeader,
			EnableResponseHeader: middlewareCfg.TraceID.EnableResponseHeader,
		}
		engine.Use(middleware.TraceID(traceCfg))
	}

	// 限流中间件：全局应用限流（在日志中间件之前，这样限流事件也会被记录）
	if limiterManager != nil && limiterManager.IsEnabled() {
		limiterCfg := limiterManager.GetConfig()
		rateLimiterCfg := middleware.DefaultRateLimiterConfig(limiterManager)

		// 跳过限流的路径
		if len(limiterCfg.SkipPaths) > 0 {
			rateLimiterCfg.SkipPaths = limiterCfg.SkipPaths
		}

		// 根据配置选择键函数
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
			// 默认：METHOD:PATH（已在 DefaultRateLimiterConfig 中设置）
		default:
			logger.Warn("yogan", "Unknown KeyFunc config, using default",
				zap.String("key_func", limiterCfg.KeyFunc))
		}

		engine.Use(middleware.RateLimiterWithConfig(rateLimiterCfg))
		logger.Debug("yogan", "✅ Rate limiter middleware globally enabled",
			zap.String("key_func", limiterCfg.KeyFunc))
	}

	// HTTP 请求日志中间件
	if middlewareCfg != nil && middlewareCfg.RequestLog != nil && middlewareCfg.RequestLog.Enable {
		requestLogCfg := middleware.RequestLogConfig{
			SkipPaths:   middlewareCfg.RequestLog.SkipPaths,
			EnableBody:  middlewareCfg.RequestLog.EnableBody,
			MaxBodySize: middlewareCfg.RequestLog.MaxBodySize,
		}
		engine.Use(middleware.RequestLogWithConfig(requestLogCfg))
	}

	// HTTP 错误日志中间件
	if httpxCfg != nil && httpxCfg.Enable {
		engine.Use(httpx.ErrorLoggingMiddleware(*httpxCfg))
	}

	// Panic 恢复中间件（总是启用）
	engine.Use(middleware.Recovery())

	// ====================================
	// 5. 注册 404/405 统一响应处理
	// ====================================
	engine.NoRoute(httpx.NoRouteHandler())
	engine.NoMethod(httpx.NoMethodHandler())

	return &HTTPServer{
		engine: engine,
		port:   cfg.Port,
		mode:   cfg.Mode,
	}
}
