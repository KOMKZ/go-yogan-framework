package governance

import (
	"context"
	"testing"
	"time"

	"github.com/KOMKZ/go-yogan-framework/logger"
)

// MockRegistry 模拟服务注册器（用于测试）
type MockRegistry struct {
	RegisterFunc        func(ctx context.Context, info *ServiceInfo) error
	DeregisterFunc      func(ctx context.Context) error
	UpdateMetadataFunc  func(ctx context.Context, metadata map[string]string) error
	IsRegisteredFunc    func() bool
	registerCallCount   int
	deregisterCallCount int
	updateMetaCallCount int
}

func (m *MockRegistry) Register(ctx context.Context, info *ServiceInfo) error {
	m.registerCallCount++
	if m.RegisterFunc != nil {
		return m.RegisterFunc(ctx, info)
	}
	return nil
}

func (m *MockRegistry) Deregister(ctx context.Context) error {
	m.deregisterCallCount++
	if m.DeregisterFunc != nil {
		return m.DeregisterFunc(ctx)
	}
	return nil
}

func (m *MockRegistry) UpdateMetadata(ctx context.Context, metadata map[string]string) error {
	m.updateMetaCallCount++
	if m.UpdateMetadataFunc != nil {
		return m.UpdateMetadataFunc(ctx, metadata)
	}
	return nil
}

func (m *MockRegistry) IsRegistered() bool {
	if m.IsRegisteredFunc != nil {
		return m.IsRegisteredFunc()
	}
	return false
}

// TestServiceInfo_Validate 测试服务信息验证
func TestServiceInfo_Validate(t *testing.T) {
	tests := []struct {
		name    string
		info    *ServiceInfo
		wantErr bool
	}{
		{
			name: "有效的服务信息",
			info: &ServiceInfo{
				ServiceName: "test-service",
				Address:     "127.0.0.1",
				Port:        8080,
			},
			wantErr: false,
		},
		{
			name: "缺少服务名称",
			info: &ServiceInfo{
				Address: "127.0.0.1",
				Port:    8080,
			},
			wantErr: true,
		},
		{
			name: "缺少地址",
			info: &ServiceInfo{
				ServiceName: "test-service",
				Port:        8080,
			},
			wantErr: true,
		},
		{
			name: "无效端口",
			info: &ServiceInfo{
				ServiceName: "test-service",
				Address:     "127.0.0.1",
				Port:        -1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.info.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}

			// 验证默认值设置
			if !tt.wantErr {
				if tt.info.TTL == 0 {
					t.Error("TTL 应该有默认值")
				}
				if tt.info.Protocol == "" {
					t.Error("Protocol 应该有默认值")
				}
				if tt.info.InstanceID == "" {
					t.Error("InstanceID 应该自动生成")
				}
			}
		})
	}
}

// TestManager_RegisterService 测试服务注册
func TestManager_RegisterService(t *testing.T) {
	mockRegistry := &MockRegistry{}
	ctxLogger := logger.GetLogger("yogan")
	manager := NewManager(mockRegistry, nil, ctxLogger)

	serviceInfo := &ServiceInfo{
		ServiceName: "test-service",
		Address:     "127.0.0.1",
		Port:        8080,
		TTL:         10,
	}

	ctx := context.Background()
	err := manager.RegisterService(ctx, serviceInfo)
	if err != nil {
		t.Fatalf("RegisterService() error = %v", err)
	}

	if mockRegistry.registerCallCount != 1 {
		t.Errorf("Register 应该被调用 1 次，实际调用了 %d 次", mockRegistry.registerCallCount)
	}

	if !manager.IsRegistered() {
		t.Error("服务应该处于已注册状态")
	}

	// 重复注册应该返回错误
	err = manager.RegisterService(ctx, serviceInfo)
	if err != ErrAlreadyRegistered {
		t.Errorf("重复注册应该返回 ErrAlreadyRegistered，实际返回 %v", err)
	}
}

// TestManager_DeregisterService 测试服务注销
func TestManager_DeregisterService(t *testing.T) {
	mockRegistry := &MockRegistry{}
	ctxLogger := logger.GetLogger("yogan")
	manager := NewManager(mockRegistry, nil, ctxLogger)

	// 先注册
	serviceInfo := &ServiceInfo{
		ServiceName: "test-service",
		Address:     "127.0.0.1",
		Port:        8080,
	}

	ctx := context.Background()
	manager.RegisterService(ctx, serviceInfo)

	// 再注销
	err := manager.DeregisterService(ctx)
	if err != nil {
		t.Fatalf("DeregisterService() error = %v", err)
	}

	if mockRegistry.deregisterCallCount != 1 {
		t.Errorf("Deregister 应该被调用 1 次，实际调用了 %d 次", mockRegistry.deregisterCallCount)
	}

	if manager.IsRegistered() {
		t.Error("服务应该处于未注册状态")
	}
}

// TestManager_UpdateMetadata 测试元数据更新
func TestManager_UpdateMetadata(t *testing.T) {
	mockRegistry := &MockRegistry{}
	ctxLogger := logger.GetLogger("yogan")
	manager := NewManager(mockRegistry, nil, ctxLogger)

	// 先注册
	serviceInfo := &ServiceInfo{
		ServiceName: "test-service",
		Address:     "127.0.0.1",
		Port:        8080,
		Metadata:    map[string]string{"version": "v1.0"},
	}

	ctx := context.Background()
	manager.RegisterService(ctx, serviceInfo)

	// 更新元数据
	newMetadata := map[string]string{
		"version": "v1.1",
		"weight":  "100",
	}

	err := manager.UpdateMetadata(ctx, newMetadata)
	if err != nil {
		t.Fatalf("UpdateMetadata() error = %v", err)
	}

	if mockRegistry.updateMetaCallCount != 1 {
		t.Errorf("UpdateMetadata 应该被调用 1 次，实际调用了 %d 次", mockRegistry.updateMetaCallCount)
	}

	// 验证元数据已更新
	info := manager.GetServiceInfo()
	if info.Metadata["version"] != "v1.1" {
		t.Errorf("version 应该是 v1.1，实际是 %s", info.Metadata["version"])
	}
	if info.Metadata["weight"] != "100" {
		t.Errorf("weight 应该是 100，实际是 %s", info.Metadata["weight"])
	}
}

// TestManager_Shutdown 测试关闭
func TestManager_Shutdown(t *testing.T) {
	mockRegistry := &MockRegistry{}
	ctxLogger := logger.GetLogger("yogan")
	manager := NewManager(mockRegistry, nil, ctxLogger)

	// 注册服务
	serviceInfo := &ServiceInfo{
		ServiceName: "test-service",
		Address:     "127.0.0.1",
		Port:        8080,
	}

	ctx := context.Background()
	manager.RegisterService(ctx, serviceInfo)

	// 关闭
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := manager.Shutdown(ctx)
	if err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}

	if manager.IsRegistered() {
		t.Error("关闭后服务应该处于未注册状态")
	}
}
