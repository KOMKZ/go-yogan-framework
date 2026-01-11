package grpc

import (
	"context"
	"fmt"
	"net"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/stats"
)

// Server gRPC 服务端封装
type Server struct {
	config         ServerConfig
	server         *grpc.Server
	logger         *logger.CtxZapLogger
	Port           int                  // 实际监听端口（用于服务注册）
	tracerProvider trace.TracerProvider // 🎯 OpenTelemetry TracerProvider（可选）
	statsHandler   stats.Handler        // 🎯 StatsHandler（用于 OTel 集成）
	interceptors   []grpc.UnaryServerInterceptor
	serverOpts     []grpc.ServerOption // 🎯 额外的 Server 选项
}

// NewServer 创建 gRPC Server（使用默认拦截器）
func NewServer(cfg ServerConfig, log *logger.CtxZapLogger) *Server {
	// 从配置读取是否启用日志（默认 true）
	enableLog := cfg.IsLogEnabled()

	// 默认拦截器链
	interceptors := []grpc.UnaryServerInterceptor{
		UnaryServerTraceInterceptor(),          // 1️⃣ TraceID 提取
		UnaryLoggerInterceptor(log, enableLog), // 2️⃣ 日志记录（可配置）
		UnaryRecoveryInterceptor(log),          // 3️⃣ Panic 恢复
	}

	return NewServerWithInterceptors(cfg, log, interceptors)
}

// NewServerWithInterceptors 创建 gRPC Server（自定义拦截器链）
// 注意：此时不会立即创建 grpc.Server，而是在 Start 时创建，以便注入 StatsHandler
func NewServerWithInterceptors(
	cfg ServerConfig,
	log *logger.CtxZapLogger,
	interceptors []grpc.UnaryServerInterceptor,
) *Server {
	return &Server{
		config:       cfg,
		logger:       log,
		Port:         cfg.Port,
		interceptors: interceptors,
		serverOpts: []grpc.ServerOption{
			grpc.MaxRecvMsgSize(cfg.MaxRecvSize * 1024 * 1024), // MB 转 Bytes
			grpc.MaxSendMsgSize(cfg.MaxSendSize * 1024 * 1024), // MB 转 Bytes
		},
	}
}

// Start 启动 gRPC Server（非阻塞）
// 🎯 在 Start 时才创建 grpc.Server，以便注入 StatsHandler
func (s *Server) Start(ctx context.Context) error {
	// 🎯 延迟创建 grpc.Server，支持 StatsHandler 注入
	if s.server == nil {
		s.buildGRPCServer()
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.config.Port))
	if err != nil {
		return fmt.Errorf("监听端口失败: %w", err)
	}

	// 获取实际监听端口（支持端口 0 自动分配）
	s.Port = lis.Addr().(*net.TCPAddr).Port
	s.logger.DebugCtx(ctx, "🚀 gRPC server started", zap.Int("port", s.Port))

	// 启动服务（非阻塞）
	go func() {
		if err := s.server.Serve(lis); err != nil {
			s.logger.ErrorCtx(ctx, "gRPC server exited abnormally", zap.Error(err))
		}
	}()

	return nil
}

// buildGRPCServer 构建 grpc.Server（在 Start 时调用）
func (s *Server) buildGRPCServer() {
	opts := make([]grpc.ServerOption, 0, len(s.serverOpts)+2)

	// 1. 添加 StatsHandler（优先级最高，必须在拦截器之前）
	if s.statsHandler != nil {
		opts = append(opts, grpc.StatsHandler(s.statsHandler))
		s.logger.DebugCtx(context.Background(), "✅ StatsHandler registered to gRPC server")
	}

	// 2. 添加拦截器链
	if len(s.interceptors) > 0 {
		opts = append(opts, grpc.ChainUnaryInterceptor(s.interceptors...))
	}

	// 3. 添加其他选项
	opts = append(opts, s.serverOpts...)

	// 创建 grpc.Server
	s.server = grpc.NewServer(opts...)

	// 启用反射（方便调试）
	if s.config.EnableReflect {
		reflection.Register(s.server)
	}
}

// Stop 优雅停止 gRPC Server
func (s *Server) Stop(ctx context.Context) {
	if s.server == nil {
		return
	}
	s.logger.DebugCtx(ctx, "⏹️  Stopping gRPC server...")
	s.server.GracefulStop()
}

// GetGRPCServer 获取原始 gRPC Server（用于注册服务实现）
// 🎯 如果 server 为 nil，先构建它
func (s *Server) GetGRPCServer() *grpc.Server {
	if s.server == nil {
		s.buildGRPCServer()
	}
	return s.server
}

// SetTracerProvider 设置 TracerProvider（在 Start 之前调用）
// 🎯 自动创建 otelgrpc.NewServerHandler
func (s *Server) SetTracerProvider(tp trace.TracerProvider) {
	s.tracerProvider = tp
	if tp != nil {
		// 创建官方 StatsHandler
		s.statsHandler = otelgrpc.NewServerHandler(
			otelgrpc.WithTracerProvider(tp),
		)
		s.logger.DebugCtx(context.Background(), "✅ TracerProvider injected into gRPC server")
	}
}

// SetMetricsHandler 设置 Metrics StatsHandler（在 Start 之前调用）
// 注意：如果已经设置了 TracerProvider，会被覆盖
// TODO: 支持同时使用 Trace 和 Metrics 的 StatsHandler（需要组合）
func (s *Server) SetMetricsHandler(handler stats.Handler) {
	if handler != nil {
		s.statsHandler = handler
		s.logger.DebugCtx(context.Background(), "✅ Metrics StatsHandler set in gRPC server")
	}
}
