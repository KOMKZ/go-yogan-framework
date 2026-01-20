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

	// Add to blacklist
	err := store.AddToBlacklist(ctx, token, 1*time.Hour)
	assert.NoError(t, err)

	// Remove blacklist entries
	err = store.RemoveFromBlacklist(ctx, token)
	assert.NoError(t, err)

	// Removed
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

	// block user
	err := store.BlacklistUserTokens(ctx, subject, 1*time.Hour)
	assert.NoError(t, err)

	// Check old token (issued time is earlier than blacklisted time)
	oldIssuedAt := time.Now().Add(-1 * time.Hour)
	blacklisted, err := store.IsUserBlacklisted(ctx, subject, oldIssuedAt)
	assert.NoError(t, err)
	assert.True(t, blacklisted)

	// Check for new token (issue time is later than blacklist time)
	time.Sleep(10 * time.Millisecond) // Ensure time difference
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

	// not blacklisted
	blacklisted, err := store.IsUserBlacklisted(ctx, subject, time.Now())
	assert.NoError(t, err)
	assert.False(t, blacklisted)

	// block user
	err = store.BlacklistUserTokens(ctx, subject, 1*time.Hour)
	assert.NoError(t, err)

	// Old token is blacklisted
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

	// Add short TTL Token
	token1 := "token1"
	err := store.AddToBlacklist(ctx, token1, 50*time.Millisecond)
	assert.NoError(t, err)

	// Add long TTL Token
	token2 := "token2"
	err = store.AddToBlacklist(ctx, token2, 1*time.Hour)
	assert.NoError(t, err)

	// waiting for cleanup
	time.Sleep(200 * time.Millisecond)

	// token1 should be cleared
	blacklisted, err := store.IsBlacklisted(ctx, token1)
	assert.NoError(t, err)
	assert.False(t, blacklisted)

	// token2 is still on the blacklist
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

	// Add expired token
	err := store.AddToBlacklist(ctx, token, 10*time.Millisecond)
	assert.NoError(t, err)

	// wait for expiration
	time.Sleep(20 * time.Millisecond)

	// Return false after expiration
	blacklisted, err := store.IsBlacklisted(ctx, token)
	assert.NoError(t, err)
	assert.False(t, blacklisted)
}

func TestMemoryTokenStore_Close(t *testing.T) {
	log := logger.NewCtxZapLogger("yogan")
	store := NewMemoryTokenStore(100*time.Millisecond, log)

	err := store.Close()
	assert.NoError(t, err)

	// Shut down cleanup after closing
	time.Sleep(200 * time.Millisecond)
}

