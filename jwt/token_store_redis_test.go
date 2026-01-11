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

func newTestRedisClient(t *testing.T) *redis.Client {
	// 使用 miniredis 模拟 Redis
	mr := miniredis.RunT(t)

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	t.Cleanup(func() {
		client.Close()
		mr.Close()
	})

	return client
}

func TestRedisTokenStore_IsBlacklisted(t *testing.T) {
	client := newTestRedisClient(t)
	log := logger.NewCtxZapLogger("yogan")
	store := NewRedisTokenStore(client, "jwt:blacklist:", log)
	defer store.Close()

	ctx := context.Background()
	token := "test-token"

	// 未加入黑名单
	blacklisted, err := store.IsBlacklisted(ctx, token)
	assert.NoError(t, err)
	assert.False(t, blacklisted)

	// 加入黑名单
	err = store.AddToBlacklist(ctx, token, 1*time.Hour)
	assert.NoError(t, err)

	// 已加入黑名单
	blacklisted, err = store.IsBlacklisted(ctx, token)
	assert.NoError(t, err)
	assert.True(t, blacklisted)
}

func TestRedisTokenStore_AddToBlacklist(t *testing.T) {
	client := newTestRedisClient(t)
	log := logger.NewCtxZapLogger("yogan")
	store := NewRedisTokenStore(client, "jwt:blacklist:", log)
	defer store.Close()

	ctx := context.Background()
	token := "test-token"

	err := store.AddToBlacklist(ctx, token, 1*time.Hour)
	assert.NoError(t, err)

	blacklisted, err := store.IsBlacklisted(ctx, token)
	assert.NoError(t, err)
	assert.True(t, blacklisted)
}

func TestRedisTokenStore_RemoveFromBlacklist(t *testing.T) {
	client := newTestRedisClient(t)
	log := logger.NewCtxZapLogger("yogan")
	store := NewRedisTokenStore(client, "jwt:blacklist:", log)
	defer store.Close()

	ctx := context.Background()
	token := "test-token"

	// 加入黑名单
	err := store.AddToBlacklist(ctx, token, 1*time.Hour)
	assert.NoError(t, err)

	// 移除黑名单
	err = store.RemoveFromBlacklist(ctx, token)
	assert.NoError(t, err)

	// 已移除
	blacklisted, err := store.IsBlacklisted(ctx, token)
	assert.NoError(t, err)
	assert.False(t, blacklisted)
}

func TestRedisTokenStore_BlacklistUserTokens(t *testing.T) {
	client := newTestRedisClient(t)
	log := logger.NewCtxZapLogger("yogan")
	store := NewRedisTokenStore(client, "jwt:blacklist:", log)
	defer store.Close()

	ctx := context.Background()
	subject := "user123"

	// 拉黑用户
	err := store.BlacklistUserTokens(ctx, subject, 1*time.Hour)
	assert.NoError(t, err)

	// 检查旧 Token（签发时间早于拉黑时间）
	oldIssuedAt := time.Now().Add(-1 * time.Hour)
	blacklisted, err := store.IsUserBlacklisted(ctx, subject, oldIssuedAt)
	assert.NoError(t, err)
	assert.True(t, blacklisted)

	// 检查新 Token（签发时间晚于拉黑时间）
	time.Sleep(10 * time.Millisecond)
	newIssuedAt := time.Now()
	blacklisted, err = store.IsUserBlacklisted(ctx, subject, newIssuedAt)
	assert.NoError(t, err)
	assert.False(t, blacklisted)
}

func TestRedisTokenStore_IsUserBlacklisted(t *testing.T) {
	client := newTestRedisClient(t)
	log := logger.NewCtxZapLogger("yogan")
	store := NewRedisTokenStore(client, "jwt:blacklist:", log)
	defer store.Close()

	ctx := context.Background()
	subject := "user123"

	// 未拉黑
	blacklisted, err := store.IsUserBlacklisted(ctx, subject, time.Now())
	assert.NoError(t, err)
	assert.False(t, blacklisted)

	// 拉黑用户
	err = store.BlacklistUserTokens(ctx, subject, 1*time.Hour)
	assert.NoError(t, err)

	// 旧 Token 被拉黑
	oldIssuedAt := time.Now().Add(-1 * time.Hour)
	blacklisted, err = store.IsUserBlacklisted(ctx, subject, oldIssuedAt)
	assert.NoError(t, err)
	assert.True(t, blacklisted)
}

func TestRedisTokenStore_Close(t *testing.T) {
	client := newTestRedisClient(t)
	log := logger.NewCtxZapLogger("yogan")
	store := NewRedisTokenStore(client, "jwt:blacklist:", log)

	err := store.Close()
	assert.NoError(t, err)
}

