package jwt

import (
	"context"
	"testing"
	"time"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/stretchr/testify/assert"
)

func TestMemoryTokenStore_IsBlacklisted(t *testing.T) {
	log := logger.NewCtxZapLogger("yogan")
	store := NewMemoryTokenStore(0, log)
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

func TestMemoryTokenStore_AddToBlacklist(t *testing.T) {
	log := logger.NewCtxZapLogger("yogan")
	store := NewMemoryTokenStore(0, log)
	defer store.Close()

	ctx := context.Background()
	token := "test-token"

	err := store.AddToBlacklist(ctx, token, 1*time.Hour)
	assert.NoError(t, err)

	blacklisted, err := store.IsBlacklisted(ctx, token)
	assert.NoError(t, err)
	assert.True(t, blacklisted)
}

func TestMemoryTokenStore_RemoveFromBlacklist(t *testing.T) {
	log := logger.NewCtxZapLogger("yogan")
	store := NewMemoryTokenStore(0, log)
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

func TestMemoryTokenStore_BlacklistUserTokens(t *testing.T) {
	log := logger.NewCtxZapLogger("yogan")
	store := NewMemoryTokenStore(0, log)
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
	time.Sleep(10 * time.Millisecond) // 确保时间差异
	newIssuedAt := time.Now()
	blacklisted, err = store.IsUserBlacklisted(ctx, subject, newIssuedAt)
	assert.NoError(t, err)
	assert.False(t, blacklisted)
}

func TestMemoryTokenStore_IsUserBlacklisted(t *testing.T) {
	log := logger.NewCtxZapLogger("yogan")
	store := NewMemoryTokenStore(0, log)
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

func TestMemoryTokenStore_Cleanup(t *testing.T) {
	log := logger.NewCtxZapLogger("yogan")
	store := NewMemoryTokenStore(100*time.Millisecond, log)
	defer store.Close()

	ctx := context.Background()

	// 添加短 TTL Token
	token1 := "token1"
	err := store.AddToBlacklist(ctx, token1, 50*time.Millisecond)
	assert.NoError(t, err)

	// 添加长 TTL Token
	token2 := "token2"
	err = store.AddToBlacklist(ctx, token2, 1*time.Hour)
	assert.NoError(t, err)

	// 等待清理
	time.Sleep(200 * time.Millisecond)

	// token1 应被清理
	blacklisted, err := store.IsBlacklisted(ctx, token1)
	assert.NoError(t, err)
	assert.False(t, blacklisted)

	// token2 仍在黑名单
	blacklisted, err = store.IsBlacklisted(ctx, token2)
	assert.NoError(t, err)
	assert.True(t, blacklisted)
}

func TestMemoryTokenStore_ExpiredToken(t *testing.T) {
	log := logger.NewCtxZapLogger("yogan")
	store := NewMemoryTokenStore(0, log)
	defer store.Close()

	ctx := context.Background()
	token := "test-token"

	// 添加已过期的 Token
	err := store.AddToBlacklist(ctx, token, 10*time.Millisecond)
	assert.NoError(t, err)

	// 等待过期
	time.Sleep(20 * time.Millisecond)

	// 过期后应返回 false
	blacklisted, err := store.IsBlacklisted(ctx, token)
	assert.NoError(t, err)
	assert.False(t, blacklisted)
}

func TestMemoryTokenStore_Close(t *testing.T) {
	log := logger.NewCtxZapLogger("yogan")
	store := NewMemoryTokenStore(100*time.Millisecond, log)

	err := store.Close()
	assert.NoError(t, err)

	// 关闭后应停止清理
	time.Sleep(200 * time.Millisecond)
}

