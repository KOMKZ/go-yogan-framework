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
	// Use miniredis to simulate Redis
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

	// not added to blacklist
	blacklisted, err := store.IsBlacklisted(ctx, token)
	assert.NoError(t, err)
	assert.False(t, blacklisted)

	// Add to blacklist
	err = store.AddToBlacklist(ctx, token, 1*time.Hour)
	assert.NoError(t, err)

	// Added to blacklist
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

	// Add to blacklist
	err := store.AddToBlacklist(ctx, token, 1*time.Hour)
	assert.NoError(t, err)

	// Remove blacklisted entries
	err = store.RemoveFromBlacklist(ctx, token)
	assert.NoError(t, err)

	// removed
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

	// block user
	err := store.BlacklistUserTokens(ctx, subject, 1*time.Hour)
	assert.NoError(t, err)

	// Check old token (issued time is earlier than blacklist time)
	oldIssuedAt := time.Now().Add(-1 * time.Hour)
	blacklisted, err := store.IsUserBlacklisted(ctx, subject, oldIssuedAt)
	assert.NoError(t, err)
	assert.True(t, blacklisted)

	// Check for new token (issuance time is later than blacklist time)
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

	// not blacklisted
	blacklisted, err := store.IsUserBlacklisted(ctx, subject, time.Now())
	assert.NoError(t, err)
	assert.False(t, blacklisted)

	// blacklist user
	err = store.BlacklistUserTokens(ctx, subject, 1*time.Hour)
	assert.NoError(t, err)

	// Old Token is blacklisted
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

