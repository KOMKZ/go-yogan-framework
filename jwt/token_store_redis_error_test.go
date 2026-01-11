package jwt

import (
	"context"
	"testing"
	"time"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

// TestRedisTokenStore_ErrorHandling 测试 Redis 错误处理
func TestRedisTokenStore_ErrorHandling(t *testing.T) {
	// 创建一个 miniredis 实例
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer client.Close()

	log := logger.NewCtxZapLogger("yogan")
	store := NewRedisTokenStore(client, "jwt:blacklist:", log)
	defer store.Close()

	ctx := context.Background()
	token := "test-token"

	// 正常添加
	err := store.AddToBlacklist(ctx, token, 1*time.Hour)
	assert.NoError(t, err)

	// 关闭 Redis 模拟错误
	mr.Close()

	// 测试各种错误情况
	t.Run("IsBlacklisted error", func(t *testing.T) {
		_, err := store.IsBlacklisted(ctx, "another-token")
		assert.Error(t, err)
	})

	t.Run("AddToBlacklist error", func(t *testing.T) {
		err := store.AddToBlacklist(ctx, "new-token", 1*time.Hour)
		assert.Error(t, err)
	})

	t.Run("RemoveFromBlacklist error", func(t *testing.T) {
		err := store.RemoveFromBlacklist(ctx, token)
		assert.Error(t, err)
	})

	t.Run("BlacklistUserTokens error", func(t *testing.T) {
		err := store.BlacklistUserTokens(ctx, "user123", 1*time.Hour)
		assert.Error(t, err)
	})

	t.Run("IsUserBlacklisted error", func(t *testing.T) {
		_, err := store.IsUserBlacklisted(ctx, "user123", time.Now())
		assert.Error(t, err)
	})
}

// TestRedisTokenStore_IsUserBlacklisted_ParseError 测试解析错误
func TestRedisTokenStore_IsUserBlacklisted_ParseError(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer client.Close()
	defer mr.Close()

	log := logger.NewCtxZapLogger("yogan")
	store := NewRedisTokenStore(client, "jwt:blacklist:", log)
	defer store.Close()

	ctx := context.Background()
	subject := "user123"

	// 手动设置一个无效的值
	key := "jwt:blacklist:user:" + subject
	err := client.Set(ctx, key, "invalid_timestamp", 1*time.Hour).Err()
	assert.NoError(t, err)

	// 尝试检查用户黑名单（应该返回解析错误）
	_, err = store.IsUserBlacklisted(ctx, subject, time.Now())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse blacklist time failed")
}

