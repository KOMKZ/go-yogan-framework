package grpc

import (
	"context"
	"fmt"
	"log"
	"net"
	"testing"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	pb "google.golang.org/grpc/examples/helloworld/helloworld"
)

// initTracer 初始化 OpenTelemetry TracerProvider（输出到 stdout）
func initTestTracer(serviceName string) (trace.TracerProvider, func(), error) {
	// 创建 stdout exporter（便于查看输出）
	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		return nil, nil, err
	}

	// 创建 Resource
	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(serviceName),
	)

	// 创建 TracerProvider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	// 设置全局 TracerProvider 和 Propagator
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	cleanup := func() {
		_ = tp.Shutdown(context.Background())
	}

	return tp, cleanup, nil
}

// TestGreeterServer 简单的 gRPC 测试服务
type TestGreeterServer struct {
	pb.UnimplementedGreeterServer
}

func (s *TestGreeterServer) SayHello(ctx context.Context, req *pb.HelloRequest) (*pb.HelloReply, error) {
	log.Printf("Server received request: name=%s", req.GetName())
	return &pb.HelloReply{Message: "Hello " + req.GetName()}, nil
}

// TestOtelGRPCPropagation_WithInterceptor 测试使用自定义拦截器的 trace 传播
func TestOtelGRPCPropagation_WithInterceptor(t *testing.T) {
	tp, cleanup, err := initTestTracer("interceptor-test")
	if err != nil {
		t.Fatalf("Failed to init tracer: %v", err)
	}
	defer cleanup()

	fmt.Println("\n============================================================")
	fmt.Println("TEST 1: Custom Interceptor Trace Propagation")
	fmt.Println("============================================================")

	// 创建监听器
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer lis.Close()

	// 创建 gRPC Server（使用我们的自定义拦截器）
	tpGetter := func() trace.TracerProvider { return tp }
	server := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			UnaryServerOtelInterceptor(tpGetter),
		),
	)
	pb.RegisterGreeterServer(server, &TestGreeterServer{})

	// 启动 server
	go func() {
		if err := server.Serve(lis); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()
	defer server.Stop()

	// 创建 gRPC Client（使用我们的自定义拦截器）
	conn, err := grpc.Dial(
		lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(
			UnaryClientOtelInterceptor(tpGetter),
		),
	)
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer conn.Close()

	client := pb.NewGreeterClient(conn)

	// 创建根 Span（模拟 HTTP 请求）
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	tracer := tp.Tracer("http-client")
	ctx, rootSpan := tracer.Start(ctx, "HTTP GET /api/test")
	fmt.Println("\n📍 创建根 Span: HTTP GET /api/test")

	// 发起 gRPC 调用
	fmt.Println("📤 发起 gRPC 调用...")
	resp, err := client.SayHello(ctx, &pb.HelloRequest{Name: "Alice"})
	if err != nil {
		t.Fatalf("gRPC call failed: %v", err)
	}

	fmt.Printf("✅ 收到响应: %s\n", resp.GetMessage())
	rootSpan.End()

	// 等待 traces 导出
	time.Sleep(200 * time.Millisecond)
	fmt.Println("\n============================================================")
	fmt.Println("查看上面的 trace 输出，验证:")
	fmt.Println("1. 是否有 3 个 spans (HTTP -> gRPC Client -> gRPC Server)")
	fmt.Println("2. 所有 spans 的 TraceID 是否相同")
	fmt.Println("3. Span 的父子关系是否正确")
	fmt.Println("============================================================")
}

// TestOtelGRPCPropagation_WithStatsHandler 测试使用官方 StatsHandler 的 trace 传播
func TestOtelGRPCPropagation_WithStatsHandler(t *testing.T) {
	tp, cleanup, err := initTestTracer("statshandler-test")
	if err != nil {
		t.Fatalf("Failed to init tracer: %v", err)
	}
	defer cleanup()

	fmt.Println("\n============================================================")
	fmt.Println("TEST 2: Official StatsHandler Trace Propagation")
	fmt.Println("============================================================")

	// 创建监听器
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer lis.Close()

	// 创建 gRPC Server（使用官方 otelgrpc.NewServerHandler）
	server := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler(
			otelgrpc.WithTracerProvider(tp),
		)),
	)
	pb.RegisterGreeterServer(server, &TestGreeterServer{})

	// 启动 server
	go func() {
		if err := server.Serve(lis); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()
	defer server.Stop()

	// 创建 gRPC Client（使用官方 otelgrpc.NewClientHandler）
	conn, err := grpc.Dial(
		lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler(
			otelgrpc.WithTracerProvider(tp),
		)),
	)
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer conn.Close()

	client := pb.NewGreeterClient(conn)

	// 创建根 Span（模拟 HTTP 请求）
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	tracer := tp.Tracer("http-client")
	ctx, rootSpan := tracer.Start(ctx, "HTTP GET /api/test")
	fmt.Println("\n📍 创建根 Span: HTTP GET /api/test")

	// 发起 gRPC 调用
	fmt.Println("📤 发起 gRPC 调用...")
	resp, err := client.SayHello(ctx, &pb.HelloRequest{Name: "Bob"})
	if err != nil {
		t.Fatalf("gRPC call failed: %v", err)
	}

	fmt.Printf("✅ 收到响应: %s\n", resp.GetMessage())
	rootSpan.End()

	// 等待 traces 导出
	time.Sleep(200 * time.Millisecond)
	fmt.Println("\n============================================================")
	fmt.Println("查看上面的 trace 输出，验证:")
	fmt.Println("1. 是否有 3 个 spans (HTTP -> gRPC Client -> gRPC Server)")
	fmt.Println("2. 所有 spans 的 TraceID 是否相同")
	fmt.Println("3. Span 的父子关系是否正确")
	fmt.Println("============================================================")
}
