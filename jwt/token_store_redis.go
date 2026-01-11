package jwt

import (
	"context"
	"fmt"
	"time"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// RedisTokenStore Redis Token 存储（生产环境）
type RedisTokenStore struct {
	client    *redis.Client
	keyPrefix string
	logger    *logger.CtxZapLogger
}

// NewRedisTokenStore 创建 Redis Token 存储
func NewRedisTokenStore(client *redis.Client, keyPrefix string, log *logger.CtxZapLogger) *RedisTokenStore {
	return &RedisTokenStore{
		client:    client,
		keyPrefix: keyPrefix,
		logger:    log,
	}
}

// IsBlacklisted 检查 Token 是否在黑名单
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

// AddToBlacklist 添加到黑名单
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

// RemoveFromBlacklist 从黑名单移除
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

// BlacklistUserTokens 添加用户所有 Token 到黑名单
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

// IsUserBlacklisted 检查用户是否被全局拉黑
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

	// 解析拉黑时间戳
	var blacklistTime int64
	_, err = fmt.Sscanf(result, "%d", &blacklistTime)
	if err != nil {
		return false, fmt.Errorf("parse blacklist time failed: %w", err)
	}

	// 如果 Token 签发时间早于拉黑时间，则认为被拉黑
	return issuedAt.Unix() < blacklistTime, nil
}

// Close 关闭连接
func (s *RedisTokenStore) Close() error {
	// Redis client 由外部管理，不在此关闭
	return nil
}

// tokenKey 生成 Token 黑名单 key
func (s *RedisTokenStore) tokenKey(token string) string {
	return s.keyPrefix + "token:" + token
}

// userKey 生成用户黑名单 key
func (s *RedisTokenStore) userKey(subject string) string {
	return s.keyPrefix + "user:" + subject
}
