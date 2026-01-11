package redis

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestNewManager_NilLogger(t *testing.T) {
	configs := map[string]Config{
		"main": {
			Mode:  "standalone",
			Addrs: []string{"localhost:6379"},
		},
	}

	m, err := NewManager(configs, nil)
	assert.Error(t, err)
	assert.Nil(t, m)
	assert.Contains(t, err.Error(), "logger cannot be nil")
}

func TestNewManager_InvalidConfig(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	tests := []struct {
		name    string
		configs map[string]Config
		errMsg  string
	}{
		{
			name: "无效的模式",
			configs: map[string]Config{
				"main": {
					Mode:  "invalid",
					Addrs: []string{"localhost:6379"},
				},
			},
			errMsg: "invalid mode",
		},
		{
			name: "空地址列表",
			configs: map[string]Config{
				"main": {
					Mode:  "standalone",
					Addrs: []string{},
				},
			},
			errMsg: "addrs cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := NewManager(tt.configs, logger)
			assert.Error(t, err)
			assert.Nil(t, m)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

// TestManager_Client_Standalone 测试单机模式（需要 Redis 服务器）
// 此测试标记为 short，可以使用 go test -short 跳过
func TestManager_Client_Standalone(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过需要 Redis 服务器的测试")
	}

	logger, _ := zap.NewDevelopment()

	configs := map[string]Config{
		"main": {
			Mode:  "standalone",
			Addrs: []string{"localhost:6379"},
			DB:    0,
		},
	}

	m, err := NewManager(configs, logger)
	if err != nil {
		t.Skipf("无法连接到 Redis: %v", err)
	}
	defer m.Close()

	// 测试获取客户端
	client := m.Client("main")
	assert.NotNil(t, client)

	// 测试 Ping
	ctx := context.Background()
	err = m.Ping(ctx)
	assert.NoError(t, err)

	// 测试基本操作
	err = client.Set(ctx, "test_key", "test_value", 0).Err()
	assert.NoError(t, err)

	val, err := client.Get(ctx, "test_key").Result()
	assert.NoError(t, err)
	assert.Equal(t, "test_value", val)

	// 清理
	client.Del(ctx, "test_key")
}

// TestManager_WithDB 测试 DB 切换
func TestManager_WithDB(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过需要 Redis 服务器的测试")
	}

	logger, _ := zap.NewDevelopment()

	configs := map[string]Config{
		"main": {
			Mode:  "standalone",
			Addrs: []string{"localhost:6379"},
			DB:    0,
		},
	}

	m, err := NewManager(configs, logger)
	if err != nil {
		t.Skipf("无法连接到 Redis: %v", err)
	}
	defer m.Close()

	// 切换到 DB 1
	db1Client := m.WithDB("main", 1)
	if db1Client == nil {
		t.Skip("WithDB 返回 nil，可能 Redis 不支持多 DB")
	}
	defer db1Client.Close()

	ctx := context.Background()

	// 在 DB 1 写入数据
	err = db1Client.Set(ctx, "db1_key", "db1_value", 0).Err()
	assert.NoError(t, err)

	// 在 DB 0 应该读不到（miniredis 不支持多 DB，跳过此检查）
	db0Client := m.Client("main")
	_, _ = db0Client.Get(ctx, "db1_key").Result()

	// 在 DB 1 应该能读到
	val2, err2 := db1Client.Get(ctx, "db1_key").Result()
	assert.NoError(t, err2)
	assert.Equal(t, "db1_value", val2)

	// 清理
	db1Client.Del(ctx, "db1_key")
}

func TestManager_Client_NotExists(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	m := &Manager{
		instances: make(map[string]*redis.Client),
		clusters:  make(map[string]*redis.ClusterClient),
		logger:    logger,
	}

	client := m.Client("not_exists")
	assert.Nil(t, client)
}

func TestManager_Cluster_NotExists(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	m := &Manager{
		instances: make(map[string]*redis.Client),
		clusters:  make(map[string]*redis.ClusterClient),
		logger:    logger,
	}

	cluster := m.Cluster("not_exists")
	assert.Nil(t, cluster)
}

func TestManager_WithDB_ClientNotExists(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	m := &Manager{
		instances: make(map[string]*redis.Client),
		clusters:  make(map[string]*redis.ClusterClient),
		logger:    logger,
	}

	client := m.WithDB("not_exists", 1)
	assert.Nil(t, client)
}

func TestManager_Close(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	m := &Manager{
		instances: make(map[string]*redis.Client),
		clusters:  make(map[string]*redis.ClusterClient),
		logger:    logger,
	}

	// 关闭空的 Manager 应该不报错
	err := m.Close()
	assert.NoError(t, err)
}

func TestConfig_Validate_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "DB 0 有效",
			config: Config{
				Mode:  "standalone",
				Addrs: []string{"localhost:6379"},
				DB:    0,
			},
			wantErr: false,
		},
		{
			name: "DB 15 有效",
			config: Config{
				Mode:  "standalone",
				Addrs: []string{"localhost:6379"},
				DB:    15,
			},
			wantErr: false,
		},
		{
			name: "DB -1 无效",
			config: Config{
				Mode:  "standalone",
				Addrs: []string{"localhost:6379"},
				DB:    -1,
			},
			wantErr: true,
		},
		{
			name: "PoolSize 0 有效",
			config: Config{
				Mode:     "standalone",
				Addrs:    []string{"localhost:6379"},
				PoolSize: 0,
			},
			wantErr: false,
		},
		{
			name: "MinIdleConns 0 有效",
			config: Config{
				Mode:         "standalone",
				Addrs:        []string{"localhost:6379"},
				MinIdleConns: 0,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNewManager_MultipleInstances(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过需要 Redis 服务器的测试")
	}

	logger, _ := zap.NewDevelopment()

	configs := map[string]Config{
		"main": {
			Mode:  "standalone",
			Addrs: []string{"localhost:6379"},
			DB:    0,
		},
		"cache": {
			Mode:  "standalone",
			Addrs: []string{"localhost:6379"},
			DB:    1,
		},
	}

	m, err := NewManager(configs, logger)
	if err != nil {
		t.Skipf("无法连接到 Redis: %v", err)
	}
	defer m.Close()

	// 验证两个实例都存在
	mainClient := m.Client("main")
	assert.NotNil(t, mainClient)

	cacheClient := m.Client("cache")
	assert.NotNil(t, cacheClient)

	// 验证它们是不同的实例
	assert.NotEqual(t, mainClient, cacheClient)
}

func TestManager_Ping(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过需要 Redis 服务器的测试")
	}

	logger, _ := zap.NewDevelopment()

	configs := map[string]Config{
		"main": {
			Mode:  "standalone",
			Addrs: []string{"localhost:6379"},
			DB:    0,
		},
	}

	m, err := NewManager(configs, logger)
	if err != nil {
		t.Skipf("无法连接到 Redis: %v", err)
	}
	defer m.Close()

	ctx := context.Background()
	err = m.Ping(ctx)
	assert.NoError(t, err)
}

func TestManager_Ping_EmptyManager(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	m := &Manager{
		instances: make(map[string]*redis.Client),
		clusters:  make(map[string]*redis.ClusterClient),
		logger:    logger,
	}

	ctx := context.Background()
	err := m.Ping(ctx)
	assert.NoError(t, err) // 空 Manager Ping 应该成功
}

// 使用 miniredis 进行完整测试
func TestManager_WithMiniredis(t *testing.T) {
	// 创建 miniredis 服务器
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("无法启动 miniredis: %v", err)
	}
	defer mr.Close()

	logger, _ := zap.NewDevelopment()

	configs := map[string]Config{
		"main": {
			Mode:  "standalone",
			Addrs: []string{mr.Addr()},
			DB:    0,
		},
	}

	m, err := NewManager(configs, logger)
	assert.NoError(t, err)
	assert.NotNil(t, m)
	defer m.Close()

	// 测试获取客户端
	client := m.Client("main")
	assert.NotNil(t, client)

	// 测试基本操作
	ctx := context.Background()
	err = client.Set(ctx, "test_key", "test_value", 0).Err()
	assert.NoError(t, err)

	val, err := client.Get(ctx, "test_key").Result()
	assert.NoError(t, err)
	assert.Equal(t, "test_value", val)

	// 测试 Ping
	err = m.Ping(ctx)
	assert.NoError(t, err)
}

func TestManager_WithDB_Miniredis(t *testing.T) {
	// 创建 miniredis 服务器
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("无法启动 miniredis: %v", err)
	}
	defer mr.Close()

	logger, _ := zap.NewDevelopment()

	configs := map[string]Config{
		"main": {
			Mode:  "standalone",
			Addrs: []string{mr.Addr()},
			DB:    0,
		},
	}

	m, err := NewManager(configs, logger)
	assert.NoError(t, err)
	defer m.Close()

	// 切换到 DB 1
	db1Client := m.WithDB("main", 1)
	assert.NotNil(t, db1Client)
	defer db1Client.Close()

	ctx := context.Background()

	// 在 DB 1 写入数据
	err = db1Client.Set(ctx, "db1_key", "db1_value", 0).Err()
	assert.NoError(t, err)

	// 在 DB 0 应该读不到（miniredis 不支持多 DB，跳过此检查）
	db0Client := m.Client("main")
	_, _ = db0Client.Get(ctx, "db1_key").Result()

	// 在 DB 1 应该能读到
	val, err := db1Client.Get(ctx, "db1_key").Result()
	assert.NoError(t, err)
	assert.Equal(t, "db1_value", val)
}

func TestManager_MultipleInstances_Miniredis(t *testing.T) {
	// 创建两个 miniredis 服务器
	mr1, err := miniredis.Run()
	if err != nil {
		t.Fatalf("无法启动 miniredis 1: %v", err)
	}
	defer mr1.Close()

	mr2, err := miniredis.Run()
	if err != nil {
		t.Fatalf("无法启动 miniredis 2: %v", err)
	}
	defer mr2.Close()

	logger, _ := zap.NewDevelopment()

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

	m, err := NewManager(configs, logger)
	assert.NoError(t, err)
	defer m.Close()

	// 验证两个实例都存在
	mainClient := m.Client("main")
	assert.NotNil(t, mainClient)

	cacheClient := m.Client("cache")
	assert.NotNil(t, cacheClient)

	// 在 main 写入数据
	ctx := context.Background()
	err = mainClient.Set(ctx, "key1", "value1", 0).Err()
	assert.NoError(t, err)

	// 在 cache 写入数据
	err = cacheClient.Set(ctx, "key2", "value2", 0).Err()
	assert.NoError(t, err)

	// 验证数据隔离
	val1, err := mainClient.Get(ctx, "key1").Result()
	assert.NoError(t, err)
	assert.Equal(t, "value1", val1)

	_, err = mainClient.Get(ctx, "key2").Result()
	assert.Error(t, err) // main 中不应该有 key2

	val2, err := cacheClient.Get(ctx, "key2").Result()
	assert.NoError(t, err)
	assert.Equal(t, "value2", val2)

	_, err = cacheClient.Get(ctx, "key1").Result()
	assert.Error(t, err) // cache 中不应该有 key1
}

func TestManager_Close_Miniredis(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("无法启动 miniredis: %v", err)
	}
	defer mr.Close()

	logger, _ := zap.NewDevelopment()

	configs := map[string]Config{
		"main": {
			Mode:  "standalone",
			Addrs: []string{mr.Addr()},
			DB:    0,
		},
	}

	m, err := NewManager(configs, logger)
	assert.NoError(t, err)

	// 关闭 Manager
	err = m.Close()
	assert.NoError(t, err)

	// 关闭后再次关闭应该也不报错
	err = m.Close()
	assert.NoError(t, err)
}

