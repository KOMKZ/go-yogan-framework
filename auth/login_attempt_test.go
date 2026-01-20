package auth

import (
	"context"
	"testing"
	"time"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getTestLoggerForAttempt() *logger.CtxZapLogger {
	mgr := logger.NewManager(logger.ManagerConfig{
		Level:         "debug",
		Encoding:      "console",
		EnableConsole: true,
	})
	return mgr.GetLogger("test")
}

func TestMemoryLoginAttemptStore_GetAttempts(t *testing.T) {
	log := getTestLoggerForAttempt()
	store := NewMemoryLoginAttemptStore(log)
	ctx := context.Background()

	t.Run("no attempts", func(t *testing.T) {
		attempts, err := store.GetAttempts(ctx, "user1")
		assert.NoError(t, err)
		assert.Equal(t, 0, attempts)
	})

	t.Run("after increment", func(t *testing.T) {
		err := store.IncrementAttempts(ctx, "user2", 1*time.Minute)
		require.NoError(t, err)

		attempts, err := store.GetAttempts(ctx, "user2")
		assert.NoError(t, err)
		assert.Equal(t, 1, attempts)
	})

	t.Run("expired attempts return zero", func(t *testing.T) {
		// 直接操作内部状态模拟过期
		store.mu.Lock()
		store.attempts["expired_user"] = &attemptRecord{
			count:     5,
			expiresAt: time.Now().Add(-1 * time.Minute), // 已过期
		}
		store.mu.Unlock()

		attempts, err := store.GetAttempts(ctx, "expired_user")
		assert.NoError(t, err)
		assert.Equal(t, 0, attempts)
	})
}

func TestMemoryLoginAttemptStore_IncrementAttempts(t *testing.T) {
	log := getTestLoggerForAttempt()
	store := NewMemoryLoginAttemptStore(log)
	ctx := context.Background()

	t.Run("first increment", func(t *testing.T) {
		err := store.IncrementAttempts(ctx, "user1", 5*time.Minute)
		assert.NoError(t, err)

		attempts, _ := store.GetAttempts(ctx, "user1")
		assert.Equal(t, 1, attempts)
	})

	t.Run("multiple increments", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			err := store.IncrementAttempts(ctx, "user2", 5*time.Minute)
			assert.NoError(t, err)
		}

		attempts, _ := store.GetAttempts(ctx, "user2")
		assert.Equal(t, 5, attempts)
	})

	t.Run("increment after expiry resets count", func(t *testing.T) {
		// 设置一个已过期的记录
		store.mu.Lock()
		store.attempts["expired_user2"] = &attemptRecord{
			count:     10,
			expiresAt: time.Now().Add(-1 * time.Minute),
		}
		store.mu.Unlock()

		// 增加尝试次数
		err := store.IncrementAttempts(ctx, "expired_user2", 5*time.Minute)
		assert.NoError(t, err)

		attempts, _ := store.GetAttempts(ctx, "expired_user2")
		assert.Equal(t, 1, attempts) // 重新从 1 开始
	})
}

func TestMemoryLoginAttemptStore_ResetAttempts(t *testing.T) {
	log := getTestLoggerForAttempt()
	store := NewMemoryLoginAttemptStore(log)
	ctx := context.Background()

	// 先增加一些尝试
	store.IncrementAttempts(ctx, "user1", 5*time.Minute)
	store.IncrementAttempts(ctx, "user1", 5*time.Minute)

	attempts, _ := store.GetAttempts(ctx, "user1")
	assert.Equal(t, 2, attempts)

	// 重置
	err := store.ResetAttempts(ctx, "user1")
	assert.NoError(t, err)

	attempts, _ = store.GetAttempts(ctx, "user1")
	assert.Equal(t, 0, attempts)
}

func TestMemoryLoginAttemptStore_IsLocked(t *testing.T) {
	log := getTestLoggerForAttempt()
	store := NewMemoryLoginAttemptStore(log)
	ctx := context.Background()
	maxAttempts := 5

	t.Run("not locked", func(t *testing.T) {
		locked, err := store.IsLocked(ctx, "user1", maxAttempts)
		assert.NoError(t, err)
		assert.False(t, locked)
	})

	t.Run("locked after max attempts", func(t *testing.T) {
		for i := 0; i < maxAttempts; i++ {
			store.IncrementAttempts(ctx, "user2", 5*time.Minute)
		}

		locked, err := store.IsLocked(ctx, "user2", maxAttempts)
		assert.NoError(t, err)
		assert.True(t, locked)
	})

	t.Run("locked when exceeded", func(t *testing.T) {
		for i := 0; i < maxAttempts+2; i++ {
			store.IncrementAttempts(ctx, "user3", 5*time.Minute)
		}

		locked, err := store.IsLocked(ctx, "user3", maxAttempts)
		assert.NoError(t, err)
		assert.True(t, locked)
	})
}

func TestMemoryLoginAttemptStore_Close(t *testing.T) {
	log := getTestLoggerForAttempt()
	store := NewMemoryLoginAttemptStore(log)

	err := store.Close()
	assert.NoError(t, err)
}

func TestCreateLoginAttemptStore(t *testing.T) {
	log := getTestLoggerForAttempt()

	t.Run("disabled returns nil", func(t *testing.T) {
		cfg := LoginAttemptConfig{Enabled: false}
		store, err := createLoginAttemptStore(cfg, nil, log)
		assert.NoError(t, err)
		assert.Nil(t, store)
	})

	t.Run("memory storage", func(t *testing.T) {
		cfg := LoginAttemptConfig{
			Enabled: true,
			Storage: "memory",
		}
		store, err := createLoginAttemptStore(cfg, nil, log)
		assert.NoError(t, err)
		assert.NotNil(t, store)
		_, ok := store.(*MemoryLoginAttemptStore)
		assert.True(t, ok)
	})

	t.Run("redis storage without client", func(t *testing.T) {
		cfg := LoginAttemptConfig{
			Enabled: true,
			Storage: "redis",
		}
		store, err := createLoginAttemptStore(cfg, nil, log)
		assert.Error(t, err)
		assert.Nil(t, store)
		assert.Contains(t, err.Error(), "redis client is required")
	})

	t.Run("unsupported storage", func(t *testing.T) {
		cfg := LoginAttemptConfig{
			Enabled: true,
			Storage: "unknown",
		}
		store, err := createLoginAttemptStore(cfg, nil, log)
		assert.Error(t, err)
		assert.Nil(t, store)
		assert.Contains(t, err.Error(), "unsupported storage type")
	})
}

func TestRedisLoginAttemptStore_GetAttempts(t *testing.T) {
	log := getTestLoggerForAttempt()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	store := NewRedisLoginAttemptStore(client, "auth:attempt:", log)
	ctx := context.Background()

	t.Run("no attempts", func(t *testing.T) {
		attempts, err := store.GetAttempts(ctx, "user1")
		assert.NoError(t, err)
		assert.Equal(t, 0, attempts)
	})

	t.Run("after increment", func(t *testing.T) {
		err := store.IncrementAttempts(ctx, "user2", 1*time.Minute)
		require.NoError(t, err)

		attempts, err := store.GetAttempts(ctx, "user2")
		assert.NoError(t, err)
		assert.Equal(t, 1, attempts)
	})

	t.Run("invalid value returns error", func(t *testing.T) {
		// Set a non-integer value
		mr.Set("auth:attempt:invalid_user", "not_a_number")

		_, err := store.GetAttempts(ctx, "invalid_user")
		assert.Error(t, err)
	})
}

func TestRedisLoginAttemptStore_IncrementAttempts(t *testing.T) {
	log := getTestLoggerForAttempt()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	store := NewRedisLoginAttemptStore(client, "auth:attempt:", log)
	ctx := context.Background()

	t.Run("first increment", func(t *testing.T) {
		err := store.IncrementAttempts(ctx, "user1", 5*time.Minute)
		assert.NoError(t, err)

		attempts, _ := store.GetAttempts(ctx, "user1")
		assert.Equal(t, 1, attempts)
	})

	t.Run("multiple increments", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			err := store.IncrementAttempts(ctx, "user2", 5*time.Minute)
			assert.NoError(t, err)
		}

		attempts, _ := store.GetAttempts(ctx, "user2")
		assert.Equal(t, 5, attempts)
	})
}

func TestRedisLoginAttemptStore_ResetAttempts(t *testing.T) {
	log := getTestLoggerForAttempt()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	store := NewRedisLoginAttemptStore(client, "auth:attempt:", log)
	ctx := context.Background()

	// Increment first
	store.IncrementAttempts(ctx, "user1", 5*time.Minute)
	store.IncrementAttempts(ctx, "user1", 5*time.Minute)

	attempts, _ := store.GetAttempts(ctx, "user1")
	assert.Equal(t, 2, attempts)

	// Reset
	err := store.ResetAttempts(ctx, "user1")
	assert.NoError(t, err)

	attempts, _ = store.GetAttempts(ctx, "user1")
	assert.Equal(t, 0, attempts)
}

func TestRedisLoginAttemptStore_IsLocked(t *testing.T) {
	log := getTestLoggerForAttempt()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	store := NewRedisLoginAttemptStore(client, "auth:attempt:", log)
	ctx := context.Background()
	maxAttempts := 5

	t.Run("not locked", func(t *testing.T) {
		locked, err := store.IsLocked(ctx, "user1", maxAttempts)
		assert.NoError(t, err)
		assert.False(t, locked)
	})

	t.Run("locked after max attempts", func(t *testing.T) {
		for i := 0; i < maxAttempts; i++ {
			store.IncrementAttempts(ctx, "user2", 5*time.Minute)
		}

		locked, err := store.IsLocked(ctx, "user2", maxAttempts)
		assert.NoError(t, err)
		assert.True(t, locked)
	})
}

func TestRedisLoginAttemptStore_Close(t *testing.T) {
	log := getTestLoggerForAttempt()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	store := NewRedisLoginAttemptStore(client, "auth:attempt:", log)

	err := store.Close()
	assert.NoError(t, err)
}
