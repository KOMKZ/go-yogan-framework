// 非 HTTP 应用（Worker/Cron）的独立健康检查 listener。
// 上游由 WorkerApplication/CronApplication 在各自服务启动后开启，下游输出与 HTTP 应用 /health 同构的
// health.Response JSON（200 healthy/degraded；503 unhealthy）。
// 设计口径（ticket hrise-server-app 000273/000290）：非 HTTP 应用的健康语义是「还能干活」而非「进程活着」，
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
type appHealthServer struct {
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

// startWorkerHealthServer 按 health.listen 配置启动独立健康 listener（worker 入口，共享实现见 startAppHealthListener）。
// enabled=false 或 listen 为空时静默跳过（HTTP 应用走主端口 /health 路由，无此需求）。
func (a *WorkerApplication) startWorkerHealthServer() error {
	server, err := startAppHealthListener(a.ctx, a.GetInjector(), a.MustGetLogger())
	if err != nil {
		return err
	}
	a.healthServer = server
	return nil
}

// startAppHealthListener 是 worker/cron 共用的独立健康 listener 启动核心：
// 读 health 配置段、按配置存在性注册组件探活项、启动 HTTP listener；
// enabled=false 或 listen 为空时返回 (nil, nil)（HTTP 应用走主端口 /health 路由，无此需求）。
func startAppHealthListener(ctx context.Context, injector *do.RootScope, log *logger.CtxZapLogger) (*appHealthServer, error) {
	loader, err := do.Invoke[*config.Loader](injector)
	if err != nil {
		return nil, fmt.Errorf("invoke config loader for health server: %w", err)
	}
	cfg, err := loadHealthConfig(loader)
	if err != nil {
		return nil, fmt.Errorf("health server config invalid: %w", err)
	}
	if !cfg.Enabled || cfg.Listen == "" {
		log.DebugCtx(ctx, "app health server disabled",
			zap.Bool("enabled", cfg.Enabled), zap.String("listen", cfg.Listen))
		return nil, nil
	}
	aggregator := resolveAppHealthAggregator(ctx, injector, cfg)
	server := &appHealthServer{
		httpServer: &http.Server{
			Addr:              cfg.Listen,
			Handler:           newWorkerHealthMux(aggregator),
			ReadHeaderTimeout: healthReadHeaderTimeout,
		},
		logger: log,
		addr:   cfg.Listen,
	}
	go func() {
		server.logger.InfoCtx(ctx, "app health server listening", zap.String("addr", server.addr))
		if serveErr := server.httpServer.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			server.logger.ErrorCtx(ctx, "app health server failed", zap.Error(serveErr))
		}
	}()
	return server, nil
}

// resolveAppHealthAggregator 优先复用 DI 中的聚合器；缺失时按配置超时现建，
// 两种路径都注册应用运行必备组件的探活项（queue broker / database / redis）。
func resolveAppHealthAggregator(ctx context.Context, injector *do.RootScope, cfg health.Config) *health.Aggregator {
	agg, err := do.Invoke[*health.Aggregator](injector)
	if err != nil || agg == nil {
		agg = health.NewAggregator(cfg.Timeout)
	}
	registerAppHealthCheckers(agg, injector)
	return agg
}

// registerAppHealthCheckers 把应用依赖组件的探活项注册进聚合器。
// 各组件按配置存在性惰性注册：未配置的组件（nil manager）跳过，绝不让健康检查因未使用的组件而失败。
func registerAppHealthCheckers(agg *health.Aggregator, injector *do.RootScope) {
	if agg == nil {
		return
	}
	// queue 探活仅在应用真的配置了队列（queue 段或 worker 的 job 段）时注册：
	// 框架 queue 配置缺省值自带 Enabled+默认 broker，无队列应用（如 cron）不注册，
	// 否则健康检查会去 ping 一个它从不使用的 broker（ticket hrise-server-app 000290 生产实测）。
	queueConfigured := false
	if loader, err := do.Invoke[*config.Loader](injector); err == nil && loader != nil {
		queueConfigured = loader.IsSet("queue") || loader.IsSet("job")
	}
	if queueConfigured {
		if cfg, err := do.Invoke[*frameworkqueue.Config](injector); err == nil && cfg != nil {
			agg.Register(frameworkqueue.NewHealthChecker(*cfg))
		}
	}
	if mgr, err := do.Invoke[*database.Manager](injector); err == nil && mgr != nil {
		agg.Register(database.NewHealthChecker(mgr))
	}
	if mgr, err := do.Invoke[*frameworkredis.Manager](injector); err == nil && mgr != nil {
		agg.Register(frameworkredis.NewHealthChecker(mgr))
	}
}

// shutdown 停止独立健康 listener；未启动时为 no-op。
func (s *appHealthServer) shutdown(ctx context.Context) error {
	if s == nil || s.httpServer == nil {
		return nil
	}
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown worker health server: %w", err)
	}
	s.logger.InfoCtx(ctx, "app health server stopped", zap.String("addr", s.addr))
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
