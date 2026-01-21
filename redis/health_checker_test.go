package redis

import (
	"context"
	"testing"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
)

func TestHealthChecker_Basic(t *testing.T) {
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

	checker := NewHealthChecker(m)
	assert.NotNil(t, checker)

	// 测试 Name
	assert.Equal(t, "redis", checker.Name())

	// 测试 Check - 正常情况
	ctx := context.Background()
	err = checker.Check(ctx)
	assert.NoError(t, err)
}

func TestHealthChecker_NilManager(t *testing.T) {
	checker := NewHealthChecker(nil)
	assert.NotNil(t, checker)

	ctx := context.Background()
	err := checker.Check(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "redis manager not initialized")
}

func TestHealthChecker_PingFailed(t *testing.T) {
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

	checker := NewHealthChecker(m)

	// 关闭 miniredis 模拟连接失败
	mr.Close()

	ctx := context.Background()
	err = checker.Check(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ping failed")
}

func TestHealthChecker_MultipleInstances(t *testing.T) {
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

	checker := NewHealthChecker(m)

	ctx := context.Background()
	err = checker.Check(ctx)
	assert.NoError(t, err)
}

func TestHealthChecker_EmptyManager(t *testing.T) {
	log := logger.GetLogger("test")

	// 空配置
	configs := map[string]Config{}

	m, err := NewManager(configs, log)
	assert.NoError(t, err)
	defer m.Close()

	checker := NewHealthChecker(m)

	ctx := context.Background()
	err = checker.Check(ctx)
	assert.NoError(t, err) // 空 manager 应该通过检查
}
