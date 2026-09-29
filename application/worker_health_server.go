// 非 HTTP 应用（Worker/Cron）的独立健康检查 listener。
// 上游由 WorkerApplication 在队列服务启动后开启，下游输出与 HTTP 应用 /health 同构的
// health.Response JSON（200 healthy/degraded；503 unhealthy）。
// 设计口径（ticket hrise-server-app 000273）：worker 的健康语义是「还能干活」而非「进程活着」，
// readiness 含组件依赖探测（queue broker redis ping / database ping / redis ping），
// 进程级探活（pgrep）作为唯一健康信号是被否决的反模式。
package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/KOMKZ/go-yogan-framework/config"
	"github.com/KOMKZ/go-yogan-framework/database"
	"github.com/KOMKZ/go-yogan-framework/health"
	"github.com/KOMKZ/go-yogan-framework/logger"
	frameworkqueue "github.com/KOMKZ/go-yogan-framework/queue"
	frameworkredis "github.com/KOMKZ/go-yogan-framework/redis"
	"github.com/samber/do/v2"
	"go.uber.org/zap"
)

// healthReadHeaderTimeout 独立 listener 只服务探活请求，包头超时取小值防慢连接占住端口。
const healthReadHeaderTimeout = 5 * time.Second

// workerHealthServer 是 worker/cron 应用的独立健康检查 HTTP listener。
type workerHealthServer struct {
	httpServer *http.Server
	logger     *logger.CtxZapLogger
	addr       string
}

// loadHealthConfig 读取 health 配置段；未配置时使用默认值并校验。
func loadHealthConfig(loader *config.Loader) (health.Config, error) {
	cfg := health.DefaultConfig()
	if loader.IsSet("health") {
		if err := loader.GetViper().UnmarshalKey("health", &cfg); err != nil {
			return health.Config{}, fmt.Errorf("decode health config: %w", err)
		}
	}
	if err := cfg.Validate(); err != nil {
		return health.Config{}, err
	}
	return cfg, nil
}

// startWorkerHealthServer 按 health.listen 配置启动独立健康 listener。
// enabled=false 或 listen 为空时静默跳过（HTTP 应用走主端口 /health 路由，无此需求）。
func (a *WorkerApplication) startWorkerHealthServer() error {
	loader, err := do.Invoke[*config.Loader](a.GetInjector())
	if err != nil {
		return fmt.Errorf("invoke config loader for health server: %w", err)
	}
	cfg, err := loadHealthConfig(loader)
	if err != nil {
		return fmt.Errorf("health server config invalid: %w", err)
	}
	if !cfg.Enabled || cfg.Listen == "" {
		a.MustGetLogger().DebugCtx(a.ctx, "worker health server disabled",
			zap.Bool("enabled", cfg.Enabled), zap.String("listen", cfg.Listen))
		return nil
	}
	aggregator := a.resolveHealthAggregator(cfg)
	server := &workerHealthServer{
		httpServer: &http.Server{
			Addr:              cfg.Listen,
			Handler:           newWorkerHealthMux(aggregator),
			ReadHeaderTimeout: healthReadHeaderTimeout,
		},
		logger: a.MustGetLogger(),
		addr:   cfg.Listen,
	}
	a.healthServer = server
	go func() {
		server.logger.InfoCtx(a.ctx, "worker health server listening", zap.String("addr", server.addr))
		if serveErr := server.httpServer.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			server.logger.ErrorCtx(a.ctx, "worker health server failed", zap.Error(serveErr))
		}
	}()
	return nil
}

// resolveHealthAggregator 优先复用 DI 中的聚合器；缺失时按配置超时现建，
// 两种路径都注册 worker 运行必备组件的探活项（queue broker / database / redis）。
func (a *WorkerApplication) resolveHealthAggregator(cfg health.Config) *health.Aggregator {
	agg, err := do.Invoke[*health.Aggregator](a.GetInjector())
	if err != nil || agg == nil {
		agg = health.NewAggregator(cfg.Timeout)
	}
	a.registerWorkerHealthCheckers(agg)
	return agg
}

// registerWorkerHealthCheckers 把 worker 依赖组件的探活项注册进聚合器。
// 各组件按配置存在性惰性注册：未配置的组件（nil manager）跳过，绝不让健康检查因未使用的组件而失败。
func (a *WorkerApplication) registerWorkerHealthCheckers(agg *health.Aggregator) {
	if agg == nil {
		return
	}
	if cfg, err := do.Invoke[*frameworkqueue.Config](a.GetInjector()); err == nil && cfg != nil {
		agg.Register(frameworkqueue.NewHealthChecker(*cfg))
	}
	if mgr, err := do.Invoke[*database.Manager](a.GetInjector()); err == nil && mgr != nil {
		agg.Register(database.NewHealthChecker(mgr))
	}
	if mgr, err := do.Invoke[*frameworkredis.Manager](a.GetInjector()); err == nil && mgr != nil {
		agg.Register(frameworkredis.NewHealthChecker(mgr))
	}
}

// shutdown 停止独立健康 listener；未启动时为 no-op。
func (s *workerHealthServer) shutdown(ctx context.Context) error {
	if s == nil || s.httpServer == nil {
		return nil
	}
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown worker health server: %w", err)
	}
	s.logger.InfoCtx(ctx, "worker health server stopped", zap.String("addr", s.addr))
	return nil
}

// newWorkerHealthMux 构建与 HTTP 应用 /health 同构的路由：
// /health 全量探测（503 on unhealthy）、/health/liveness 进程活性、/health/readiness 依赖就绪。
func newWorkerHealthMux(aggregator *health.Aggregator) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		response := aggregator.Check(r.Context())
		status := http.StatusOK
		if response.Status == health.StatusUnhealthy {
			status = http.StatusServiceUnavailable
		}
		writeWorkerHealthJSON(w, status, response)
	})
	mux.HandleFunc("GET /health/liveness", func(w http.ResponseWriter, r *http.Request) {
		// liveness 只回答进程与事件循环是否存活，不探测外部依赖。
		writeWorkerHealthJSON(w, http.StatusOK, map[string]string{"status": "alive"})
	})
	mux.HandleFunc("GET /health/readiness", func(w http.ResponseWriter, r *http.Request) {
		response := aggregator.Check(r.Context())
		status := http.StatusOK
		if response.Status != health.StatusHealthy {
			status = http.StatusServiceUnavailable
		}
		writeWorkerHealthJSON(w, status, map[string]string{"status": string(response.Status)})
	})
	return mux
}

// writeWorkerHealthJSON 输出健康响应 JSON；与 middleware.RegisterHealthRoutes 的响应契约一致。
func writeWorkerHealthJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		// 探活响应无需重试语义；编码失败仅代表客户端连接异常。
		return
	}
}
