package jwt

import (
	"context"
	"fmt"
	"time"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// RedisTokenStore Redis token storage (production environment)
type RedisTokenStore struct {
	client    *redis.Client
	keyPrefix string
	logger    *logger.CtxZapLogger
}

// NewRedisTokenStore creates Redis Token storage
func NewRedisTokenStore(client *redis.Client, keyPrefix string, log *logger.CtxZapLogger) *RedisTokenStore {
	return &RedisTokenStore{
		client:    client,
		keyPrefix: keyPrefix,
		logger:    log,
	}
}

// Check if Token is in blacklist
func (s *RedisTokenStore) IsBlacklisted(ctx context.Context, token string) (bool, error) {
	key := s.tokenKey(token)
	exists, err := s.client.Exists(ctx, key).Result()
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to check token blacklist",
			zap.Error(err),
			zap.String("key", key),
		)
		return false, fmt.Errorf("check blacklist failed: %w", err)
	}

	return exists > 0, nil
}

// AddToBlacklist Add to blacklist
func (s *RedisTokenStore) AddToBlacklist(ctx context.Context, token string, ttl time.Duration) error {
	key := s.tokenKey(token)
	err := s.client.Set(ctx, key, "1", ttl).Err()
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to add token to blacklist",
			zap.Error(err),
			zap.String("key", key),
			zap.Duration("ttl", ttl),
		)
		return fmt.Errorf("add to blacklist failed: %w", err)
	}

	s.logger.DebugCtx(ctx, "token added to blacklist",
		zap.String("key", key),
		zap.Duration("ttl", ttl),
	)

	return nil
}

// RemoveFromBlacklist Remove from blacklist
func (s *RedisTokenStore) RemoveFromBlacklist(ctx context.Context, token string) error {
	key := s.tokenKey(token)
	err := s.client.Del(ctx, key).Err()
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to remove token from blacklist",
			zap.Error(err),
			zap.String("key", key),
		)
		return fmt.Errorf("remove from blacklist failed: %w", err)
	}

	s.logger.DebugCtx(ctx, "token removed from blacklist",
		zap.String("key", key),
	)

	return nil
}

// Add all user tokens to the blacklist
func (s *RedisTokenStore) BlacklistUserTokens(ctx context.Context, subject string, ttl time.Duration) error {
	key := s.userKey(subject)
	timestamp := time.Now().Unix()

	err := s.client.Set(ctx, key, timestamp, ttl).Err()
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to blacklist user tokens",
			zap.Error(err),
			zap.String("subject", subject),
		)
		return fmt.Errorf("blacklist user tokens failed: %w", err)
	}

	s.logger.InfoCtx(ctx, "user tokens blacklisted",
		zap.String("subject", subject),
		zap.Int64("timestamp", timestamp),
	)

	return nil
}

// Check if user is globally blacklisted
func (s *RedisTokenStore) IsUserBlacklisted(ctx context.Context, subject string, issuedAt time.Time) (bool, error) {
	key := s.userKey(subject)
	result, err := s.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to check user blacklist",
			zap.Error(err),
			zap.String("subject", subject),
		)
		return false, fmt.Errorf("check user blacklist failed: %w", err)
	}

	// Parse blocklist timestamp
	var blacklistTime int64
	_, err = fmt.Sscanf(result, "%d", &blacklistTime)
	if err != nil {
		return false, fmt.Errorf("parse blacklist time failed: %w", err)
	}

	// If the token issuance time is earlier than the blacklist time, it is considered blacklisted
	return issuedAt.Unix() < blacklistTime, nil
}

// Close connection
func (s *RedisTokenStore) Close() error {
	// The Redis client is managed externally and is not closed here
	return nil
}

// tokenKey generates the Token blacklist key
func (s *RedisTokenStore) tokenKey(token string) string {
	return s.keyPrefix + "token:" + token
}

// generate user blacklist key
func (s *RedisTokenStore) userKey(subject string) string {
	return s.keyPrefix + "user:" + subject
}
