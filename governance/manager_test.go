package governance

import (
	"context"
	"testing"
	"time"

	"github.com/KOMKZ/go-yogan-framework/logger"
)

// MockRegistry mock service registry (for testing)
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

// TestServiceInfo_Validate test service information validation
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

			// Verify default value settings
			if !tt.wantErr {
				if tt.info.TTL == 0 {
					t.Error("TTL TTL should have a default value")
				}
				if tt.info.Protocol == "" {
					t.Error("Protocol The protocol should have a default value")
				}
				if tt.info.InstanceID == "" {
					t.Error("InstanceID InstanceID should be auto-generated")
				}
			}
		})
	}
}

// TestManager_RegisterService test service registration
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
		t.Errorf("Register should be called 1 time, actually called %d times", mockRegistry.registerCallCount)
	}

	if !manager.IsRegistered() {
		t.Error("The service should be in a registered state.")
	}

	// Duplicate registration should return an error
	err = manager.RegisterService(ctx, serviceInfo)
	if err != ErrAlreadyRegistered {
		t.Errorf("Duplicate registration should return ErrAlreadyRegistered, actually returned %v", err)
	}
}

// TestManager_DeregisterService test service deregistration
func TestManager_DeregisterService(t *testing.T) {
	mockRegistry := &MockRegistry{}
	ctxLogger := logger.GetLogger("yogan")
	manager := NewManager(mockRegistry, nil, ctxLogger)

	// Register first
	serviceInfo := &ServiceInfo{
		ServiceName: "test-service",
		Address:     "127.0.0.1",
		Port:        8080,
	}

	ctx := context.Background()
	manager.RegisterService(ctx, serviceInfo)

	// Re-log out
	err := manager.DeregisterService(ctx)
	if err != nil {
		t.Fatalf("DeregisterService() error = %v", err)
	}

	if mockRegistry.deregisterCallCount != 1 {
		t.Errorf("Deregister should be called 1 time, actually called %d times", mockRegistry.deregisterCallCount)
	}

	if manager.IsRegistered() {
		t.Error("The service should be in an unregistered state.")
	}
}

// TestManager_UpdateMetadata metadata update test
func TestManager_UpdateMetadata(t *testing.T) {
	mockRegistry := &MockRegistry{}
	ctxLogger := logger.GetLogger("yogan")
	manager := NewManager(mockRegistry, nil, ctxLogger)

	// Register first
	serviceInfo := &ServiceInfo{
		ServiceName: "test-service",
		Address:     "127.0.0.1",
		Port:        8080,
		Metadata:    map[string]string{"version": "v1.0"},
	}

	ctx := context.Background()
	manager.RegisterService(ctx, serviceInfo)

	// Update metadata
	newMetadata := map[string]string{
		"version": "v1.1",
		"weight":  "100",
	}

	err := manager.UpdateMetadata(ctx, newMetadata)
	if err != nil {
		t.Fatalf("UpdateMetadata() error = %v", err)
	}

	if mockRegistry.updateMetaCallCount != 1 {
		t.Errorf("UpdateMetadata should be called 1 time, actually called %d times", mockRegistry.updateMetaCallCount)
	}

	// Verify metadata has been updated
	info := manager.GetServiceInfo()
	if info.Metadata["version"] != "v1.1" {
		t.Errorf("version should be v1.1, but it is actually %s", info.Metadata["version"])
	}
	if info.Metadata["weight"] != "100" {
		t.Errorf("weight should be 100, actually it is %s", info.Metadata["weight"])
	}
}

// TestManager Shutdown test
func TestManager_Shutdown(t *testing.T) {
	mockRegistry := &MockRegistry{}
	ctxLogger := logger.GetLogger("yogan")
	manager := NewManager(mockRegistry, nil, ctxLogger)

	// Register service
	serviceInfo := &ServiceInfo{
		ServiceName: "test-service",
		Address:     "127.0.0.1",
		Port:        8080,
	}

	ctx := context.Background()
	manager.RegisterService(ctx, serviceInfo)

	// close
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := manager.Shutdown(ctx)
	if err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}

	if manager.IsRegistered() {
		t.Error("The service should be in an unregistered state after shutdown.")
	}
}

// TestManager_DeregisterService_NotRegistered test deregistration when not registered
func TestManager_DeregisterService_NotRegistered(t *testing.T) {
	mockRegistry := &MockRegistry{}
	ctxLogger := logger.GetLogger("yogan")
	manager := NewManager(mockRegistry, nil, ctxLogger)

	ctx := context.Background()
	err := manager.DeregisterService(ctx)
	if err != ErrNotRegistered {
		t.Errorf("Expected ErrNotRegistered, got %v", err)
	}
}

// TestManager_UpdateMetadata_NotRegistered test update metadata when not registered
func TestManager_UpdateMetadata_NotRegistered(t *testing.T) {
	mockRegistry := &MockRegistry{}
	ctxLogger := logger.GetLogger("yogan")
	manager := NewManager(mockRegistry, nil, ctxLogger)

	ctx := context.Background()
	err := manager.UpdateMetadata(ctx, map[string]string{"key": "value"})
	if err != ErrNotRegistered {
		t.Errorf("Expected ErrNotRegistered, got %v", err)
	}
}

// TestManager_PerformHealthCheck test health check
func TestManager_PerformHealthCheck(t *testing.T) {
	mockRegistry := &MockRegistry{}
	ctxLogger := logger.GetLogger("yogan")
	manager := NewManager(mockRegistry, nil, ctxLogger)

	ctx := context.Background()
	err := manager.PerformHealthCheck(ctx)
	if err != nil {
		t.Errorf("PerformHealthCheck() error = %v", err)
	}
}

// TestManager_GetHealthStatus test get health status
func TestManager_GetHealthStatus(t *testing.T) {
	mockRegistry := &MockRegistry{}
	ctxLogger := logger.GetLogger("yogan")
	manager := NewManager(mockRegistry, nil, ctxLogger)

	status := manager.GetHealthStatus()
	if !status.Healthy {
		t.Error("Default health status should be healthy")
	}
}

// TestNewManager_NilLogger test NewManager with nil logger
func TestNewManager_NilLogger(t *testing.T) {
	mockRegistry := &MockRegistry{}
	manager := NewManager(mockRegistry, nil, nil)
	if manager == nil {
		t.Error("NewManager should not return nil")
	}
}

// TestServiceInfo_GetFullAddress test GetFullAddress
func TestServiceInfo_GetFullAddress(t *testing.T) {
	info := &ServiceInfo{
		Address: "192.168.1.100",
		Port:    9002,
	}

	expected := "192.168.1.100:9002"
	if info.GetFullAddress() != expected {
		t.Errorf("Expected %s, got %s", expected, info.GetFullAddress())
	}
}
