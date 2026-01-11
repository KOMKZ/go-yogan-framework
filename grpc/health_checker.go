package grpc

import (
	"context"
	"fmt"
)

// HealthChecker gRPC 健康检查器
type HealthChecker struct {
	server        *Server
	clientManager *ClientManager
}

// NewHealthChecker 创建 gRPC 健康检查器
func NewHealthChecker(server *Server, clientManager *ClientManager) *HealthChecker {
	return &HealthChecker{
		server:        server,
		clientManager: clientManager,
	}
}

// Name 检查项名称
func (h *HealthChecker) Name() string {
	return "grpc"
}

// Check 执行健康检查
func (h *HealthChecker) Check(ctx context.Context) error {
	// gRPC 组件主要检查是否正常初始化即可
	// Server 和 ClientManager 都可能为 nil（取决于配置）
	
	if h.server == nil && h.clientManager == nil {
		return fmt.Errorf("grpc component not initialized")
	}

	// gRPC Server 健康状态通过是否成功启动来判断
	// gRPC Client 健康状态通过是否成功创建连接来判断
	// 这里只做基础检查，不执行实际的 RPC 调用

	return nil
}

