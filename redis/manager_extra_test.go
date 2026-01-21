package redis

import (
	"context"
	"testing"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
)

func TestManager_GetInstanceNames(t *testing.T) {
	mr1, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Unable to start miniredis: %v", err)
	}
	defer mr1.Close()

	mr2, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Unable to start miniredis: %v", err)
	}
	defer mr2.Close()

	log := logger.GetLogger("test")

	configs := map[string]Config{
		"main": {
			Mode:  "standalone",
			Addrs: []string{mr1.Addr()},
			DB:    0,
		},
		"cache": {
			Mode:  "standalone",
			Addrs: []string{mr2.Addr()},
			DB:    0,
		},
	}

	m, err := NewManager(configs, log)
	assert.NoError(t, err)
	defer m.Close()

	names := m.GetInstanceNames()
	assert.Len(t, names, 2)
	assert.Contains(t, names, "main")
	assert.Contains(t, names, "cache")
}

func TestManager_GetClusterNames(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Unable to start miniredis: %v", err)
	}
	defer mr.Close()

	log := logger.GetLogger("test")

	configs := map[string]Config{
		"main": {
			Mode:  "standalone",
			Addrs: []string{mr.Addr()},
			DB:    0,
		},
	}

	m, err := NewManager(configs, log)
	assert.NoError(t, err)
	defer m.Close()

	// 没有 cluster 实例
	clusterNames := m.GetClusterNames()
	assert.Len(t, clusterNames, 0)
}

func TestManager_Shutdown(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Unable to start miniredis: %v", err)
	}
	defer mr.Close()

	log := logger.GetLogger("test")

	configs := map[string]Config{
		"main": {
			Mode:  "standalone",
			Addrs: []string{mr.Addr()},
			DB:    0,
		},
	}

	m, err := NewManager(configs, log)
	assert.NoError(t, err)

	// Shutdown 应该和 Close 一样工作
	err = m.Shutdown()
	assert.NoError(t, err)
}

func TestManager_EmptyInstanceNames(t *testing.T) {
	log := logger.GetLogger("test")

	// 创建空配置
	configs := map[string]Config{}

	m, err := NewManager(configs, log)
	assert.NoError(t, err)
	defer m.Close()

	names := m.GetInstanceNames()
	assert.Len(t, names, 0)

	clusterNames := m.GetClusterNames()
	assert.Len(t, clusterNames, 0)
}

func TestManager_WithDB_NotFound(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Unable to start miniredis: %v", err)
	}
	defer mr.Close()

	log := logger.GetLogger("test")

	configs := map[string]Config{
		"main": {
			Mode:  "standalone",
			Addrs: []string{mr.Addr()},
			DB:    0,
		},
	}

	m, err := NewManager(configs, log)
	assert.NoError(t, err)
	defer m.Close()

	// 尝试获取不存在的实例
	client := m.WithDB("nonexistent", 1)
	assert.Nil(t, client)
}

func TestManager_Ping_Success(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Unable to start miniredis: %v", err)
	}
	defer mr.Close()

	log := logger.GetLogger("test")

	configs := map[string]Config{
		"main": {
			Mode:  "standalone",
			Addrs: []string{mr.Addr()},
			DB:    0,
		},
	}

	m, err := NewManager(configs, log)
	assert.NoError(t, err)
	defer m.Close()

	// Ping 成功
	ctx := context.Background()
	err = m.Ping(ctx)
	assert.NoError(t, err)
}

func TestManager_Ping_Empty(t *testing.T) {
	log := logger.GetLogger("test")

	// 空配置
	configs := map[string]Config{}

	m, err := NewManager(configs, log)
	assert.NoError(t, err)
	defer m.Close()

	// 空 manager Ping
	ctx := context.Background()
	err = m.Ping(ctx)
	assert.NoError(t, err)
}

func TestManager_Cluster_NotFound(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Unable to start miniredis: %v", err)
	}
	defer mr.Close()

	log := logger.GetLogger("test")

	configs := map[string]Config{
		"main": {
			Mode:  "standalone",
			Addrs: []string{mr.Addr()},
			DB:    0,
		},
	}

	m, err := NewManager(configs, log)
	assert.NoError(t, err)
	defer m.Close()

	// 获取不存在的 cluster
	cluster := m.Cluster("nonexistent")
	assert.Nil(t, cluster)
}

func TestManager_Client_NotFound(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Unable to start miniredis: %v", err)
	}
	defer mr.Close()

	log := logger.GetLogger("test")

	configs := map[string]Config{
		"main": {
			Mode:  "standalone",
			Addrs: []string{mr.Addr()},
			DB:    0,
		},
	}

	m, err := NewManager(configs, log)
	assert.NoError(t, err)
	defer m.Close()

	// 获取不存在的 client
	client := m.Client("nonexistent")
	assert.Nil(t, client)
}

func TestManager_CreateClient_ConnectionFailed(t *testing.T) {
	log := logger.GetLogger("test")

	// 使用不存在的地址
	configs := map[string]Config{
		"main": {
			Mode:         "standalone",
			Addrs:        []string{"localhost:59999"}, // 不存在的端口
			DB:           0,
			DialTimeout:  100000000, // 100ms
			ReadTimeout:  100000000,
			WriteTimeout: 100000000,
		},
	}

	m, err := NewManager(configs, log)
	assert.Error(t, err)
	assert.Nil(t, m)
	assert.Contains(t, err.Error(), "ping failed")
}

func TestManager_Ping_Failed(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Unable to start miniredis: %v", err)
	}

	log := logger.GetLogger("test")

	configs := map[string]Config{
		"main": {
			Mode:  "standalone",
			Addrs: []string{mr.Addr()},
			DB:    0,
		},
	}

	m, err := NewManager(configs, log)
	assert.NoError(t, err)

	// 关闭 miniredis
	mr.Close()

	// Ping 应该失败
	ctx := context.Background()
	err = m.Ping(ctx)
	assert.Error(t, err)
}

func TestManager_WithDB_Success(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Unable to start miniredis: %v", err)
	}
	defer mr.Close()

	log := logger.GetLogger("test")

	configs := map[string]Config{
		"main": {
			Mode:  "standalone",
			Addrs: []string{mr.Addr()},
			DB:    0,
		},
	}

	m, err := NewManager(configs, log)
	assert.NoError(t, err)
	defer m.Close()

	// WithDB 切换数据库
	client := m.WithDB("main", 1)
	assert.NotNil(t, client)
}
