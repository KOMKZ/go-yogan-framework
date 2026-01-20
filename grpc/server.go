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

// Server gRPC service encapsulation
type Server struct {
	config         ServerConfig
	server         *grpc.Server
	logger         *logger.CtxZapLogger
	Port           int                  // Actual listening port (for service registration)
	tracerProvider trace.TracerProvider // 🎯 OpenTelemetry TracerProvider (optional)
	statsHandler   stats.Handler        // 🎯 StatsHandler (for OTel integration)
	interceptors   []grpc.UnaryServerInterceptor
	serverOpts     []grpc.ServerOption // 🎯 Additional Server options
}

// Create gRPC Server (using default interceptors)
func NewServer(cfg ServerConfig, log *logger.CtxZapLogger) *Server {
	// Read from configuration whether logging is enabled (default true)
	enableLog := cfg.IsLogEnabled()

	// Default interceptor chain
	interceptors := []grpc.UnaryServerInterceptor{
		UnaryServerTraceInterceptor(),          // 1️⃣ Extract TraceID
		UnaryLoggerInterceptor(log, enableLog), // 2️⃣ Logging (configurable)
		UnaryRecoveryInterceptor(log),          // 3️⃣ Panic Recovery
	}

	return NewServerWithInterceptors(cfg, log, interceptors)
}

// Create gRPC Server (custom interceptor chain)
// Note: A grpc.Server is not created immediately at this point; it is created when Start is called to allow injection of StatsHandler
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
			grpc.MaxRecvMsgSize(cfg.MaxRecvSize * 1024 * 1024), // Convert MB to bytes
			grpc.MaxSendMsgSize(cfg.MaxSendSize * 1024 * 1024), // MB to Bytes
		},
	}
}

// Start non-blocking gRPC Server
// 🎯 Create grpc.Server only at Start to inject StatsHandler
func (s *Server) Start(ctx context.Context) error {
	// 🎯 Delayed creation of grpc.Server to support injection of StatsHandler
	if s.server == nil {
		s.buildGRPCServer()
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.config.Port))
	if err != nil {
		return fmt.Errorf("Failed to listen on port: %w: %w", err)
	}

	// Get the actual listening port (support automatic allocation for port 0)
	s.Port = lis.Addr().(*net.TCPAddr).Port
	s.logger.DebugCtx(ctx, "🚀 gRPC server started", zap.Int("port", s.Port))

	// Start service (non-blocking)
	go func() {
		if err := s.server.Serve(lis); err != nil {
			s.logger.ErrorCtx(ctx, "gRPC server exited abnormally", zap.Error(err))
		}
	}()

	return nil
}

// buildGRPCServer Builds grpc.Server (called in Start)
func (s *Server) buildGRPCServer() {
	opts := make([]grpc.ServerOption, 0, len(s.serverOpts)+2)

	// 1. Add StatsHandler (highest priority, must be before interceptors)
	if s.statsHandler != nil {
		opts = append(opts, grpc.StatsHandler(s.statsHandler))
		s.logger.DebugCtx(context.Background(), "✅ StatsHandler registered to gRPC server")
	}

	// Add interceptor chain
	if len(s.interceptors) > 0 {
		opts = append(opts, grpc.ChainUnaryInterceptor(s.interceptors...))
	}

	// 3. Add other options
	opts = append(opts, s.serverOpts...)

	// Create gRPC.Server
	s.server = grpc.NewServer(opts...)

	// Enable reflection (for convenient debugging)
	if s.config.EnableReflect {
		reflection.Register(s.server)
	}
}

// Shut down gRPC Server gracefully
func (s *Server) Stop(ctx context.Context) {
	if s.server == nil {
		return
	}
	s.logger.DebugCtx(ctx, "⏹️  Stopping gRPC server...")
	s.server.GracefulStop()
}

// GetGRPCServer Obtain the original gRPC server (for registering service implementations)
// 🎯 If server is nil, build it first
func (s *Server) GetGRPCServer() *grpc.Server {
	if s.server == nil {
		s.buildGRPCServer()
	}
	return s.server
}

// SetTracerProvider sets the TracerProvider (call before Start)
// 🎯 Automatically create otelgrpc.NewServerHandler
func (s *Server) SetTracerProvider(tp trace.TracerProvider) {
	s.tracerProvider = tp
	if tp != nil {
		// Create official StatsHandler
		s.statsHandler = otelgrpc.NewServerHandler(
			otelgrpc.WithTracerProvider(tp),
		)
		s.logger.DebugCtx(context.Background(), "✅ TracerProvider injected into gRPC server")
	}
}

// SetMetricsHandler sets the Metrics StatsHandler (called before Start)
// Note: If a TracerProvider is already set, it will be overridden
// TODO: Support using StatsHandler with both Trace and Metrics (requires composition)
func (s *Server) SetMetricsHandler(handler stats.Handler) {
	if handler != nil {
		s.statsHandler = handler
		s.logger.DebugCtx(context.Background(), "✅ Metrics StatsHandler set in gRPC server")
	}
}
