//go:build integration
// +build integration

package redis

import (
	"context"
	"testing"
	"time"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/stretchr/testify/assert"
)

// 集成测试 - 需要真实 Redis 运行在 localhost:6379
// 运行: go test -tags=integration ./redis/...

func TestManager_RealRedis_Integration(t *testing.T) {
	log := logger.GetLogger("test")

	configs := map[string]Config{
		"main": {
			Mode:         "standalone",
			Addrs:        []string{"localhost:6379"},
			DB:           0,
			Password:     "",
			PoolSize:     10,
			MinIdleConns: 5,
			MaxRetries:   3,
			DialTimeout:  5 * time.Second,
			ReadTimeout:  3 * time.Second,
			WriteTimeout: 3 * time.Second,
		},
	}

	m, err := NewManager(configs, log)
	if err != nil {
		t.Skipf("Skipping integration test: %v", err)
	}
	defer m.Close()

	// 测试基本操作
	ctx := context.Background()
	client := m.Client("main")
	assert.NotNil(t, client)

	// Set/Get
	err = client.Set(ctx, "test_key", "test_value", time.Minute).Err()
	assert.NoError(t, err)

	val, err := client.Get(ctx, "test_key").Result()
	assert.NoError(t, err)
	assert.Equal(t, "test_value", val)

	// Cleanup
	client.Del(ctx, "test_key")

	// 测试 Ping
	err = m.Ping(ctx)
	assert.NoError(t, err)

	// 测试 HealthChecker
	checker := NewHealthChecker(m)
	err = checker.Check(ctx)
	assert.NoError(t, err)
}

func TestManager_RealRedis_WithDB_Integration(t *testing.T) {
	log := logger.GetLogger("test")

	configs := map[string]Config{
		"main": {
			Mode:         "standalone",
			Addrs:        []string{"localhost:6379"},
			DB:           0,
			Password:     "",
			PoolSize:     10,
			MinIdleConns: 5,
			MaxRetries:   3,
			DialTimeout:  5 * time.Second,
			ReadTimeout:  3 * time.Second,
			WriteTimeout: 3 * time.Second,
		},
	}

	m, err := NewManager(configs, log)
	if err != nil {
		t.Skipf("Skipping integration test: %v", err)
	}
	defer m.Close()

	// 测试 WithDB
	ctx := context.Background()
	clientDB1 := m.WithDB("main", 1)
	assert.NotNil(t, clientDB1)

	// 在 DB 1 中设置值
	err = clientDB1.Set(ctx, "db1_key", "db1_value", time.Minute).Err()
	assert.NoError(t, err)

	// 验证
	val, err := clientDB1.Get(ctx, "db1_key").Result()
	assert.NoError(t, err)
	assert.Equal(t, "db1_value", val)

	// Cleanup
	clientDB1.Del(ctx, "db1_key")
}
