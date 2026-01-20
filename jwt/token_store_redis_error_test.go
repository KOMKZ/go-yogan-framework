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

// TestRedisTokenStore_ErrorHandling test Redis error handling
func TestRedisTokenStore_ErrorHandling(t *testing.T) {
	// Create a miniredis instance
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

	// Add normally
	err := store.AddToBlacklist(ctx, token, 1*time.Hour)
	assert.NoError(t, err)

	// Close Redis simulation error
	mr.Close()

	// Test various error cases
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

// TestRedisTokenStore_IsUserBlacklisted_ParseError Parse error test
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

	// Manually set an invalid value
	key := "jwt:blacklist:user:" + subject
	err := client.Set(ctx, key, "invalid_timestamp", 1*time.Hour).Err()
	assert.NoError(t, err)

	// Try to check user blacklist (should return parsing error)
	_, err = store.IsUserBlacklisted(ctx, subject, time.Now())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse blacklist time failed")
}

