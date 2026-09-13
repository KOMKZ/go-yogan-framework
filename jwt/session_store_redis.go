// 本文件实现 JWT session 的 Redis 存储。
// Redis client 由应用层 redis.Manager 管理，本实现只消费连接并维护 session key、用户索引和 token id 索引。
// 维护时不得在这里读取 DSN、创建连接或解释业务权限；这些职责分别属于应用配置、Redis manager 和消费方业务层。
package jwt

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/KOMKZ/go-yogan-framework/logger"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// RedisSessionStore stores JWT sessions in Redis.
type RedisSessionStore struct {
	client    goredis.UniversalClient
	keyPrefix string
	logger    *logger.CtxZapLogger
}

// NewRedisSessionStore creates a Redis-backed JWT session store.
func NewRedisSessionStore(client goredis.UniversalClient, cfg SessionConfig, log *logger.CtxZapLogger) *RedisSessionStore {
	return &RedisSessionStore{
		client:    client,
		keyPrefix: strings.TrimRight(cfg.KeyPrefix, ":") + ":",
		logger:    log,
	}
}

func (s *RedisSessionStore) CreateSession(ctx context.Context, session Session) error {
	if err := validateSession(session); err != nil {
		return err
	}
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshal jwt session failed: %w", err)
	}
	ttl := time.Until(session.RefreshExpiresAt)
	if ttl <= 0 {
		return ErrTokenExpired
	}
	pipe := s.client.TxPipeline()
	pipe.Set(ctx, s.sessionKey(session.SID), data, ttl)
	pipe.ZAdd(ctx, s.userSessionsKey(session.Subject), goredis.Z{Score: float64(session.LastSeenAt.UnixMilli()), Member: session.SID})
	pipe.Expire(ctx, s.userSessionsKey(session.Subject), ttl)
	pipe.Set(ctx, s.accessKey(session.CurrentAccessID), session.SID, time.Until(session.ExpiresAt))
	pipe.Set(ctx, s.refreshKey(session.CurrentRefreshID), session.SID, ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("create jwt session failed: %w", err)
	}
	s.logger.DebugCtx(ctx, "jwt session created", zap.String("sid", session.SID), zap.String("subject", session.Subject))
	return nil
}

func (s *RedisSessionStore) GetSession(ctx context.Context, sid string) (*Session, error) {
	data, err := s.client.Get(ctx, s.sessionKey(sid)).Bytes()
	if err == goredis.Nil {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get jwt session failed: %w", err)
	}
	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("unmarshal jwt session failed: %w", err)
	}
	if isSessionExpired(session, time.Now()) {
		return nil, ErrSessionNotFound
	}
	return cloneSession(session), nil
}

func (s *RedisSessionStore) ListSubjectSessions(ctx context.Context, subject string) ([]Session, error) {
	sids, err := s.client.ZRevRange(ctx, s.userSessionsKey(subject), 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("list jwt subject sessions failed: %w", err)
	}
	sessions := make([]Session, 0, len(sids))
	staleSIDs := make([]interface{}, 0)
	for _, sid := range sids {
		session, err := s.GetSession(ctx, sid)
		if err == ErrSessionNotFound {
			staleSIDs = append(staleSIDs, sid)
			continue
		}
		if err != nil {
			return nil, err
		}
		if session.Status != SessionStatusActive {
			if err := s.deleteSession(ctx, *session); err != nil {
				return nil, err
			}
			continue
		}
		sessions = append(sessions, *session)
	}
	if len(staleSIDs) > 0 {
		_ = s.client.ZRem(ctx, s.userSessionsKey(subject), staleSIDs...).Err()
	}
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastSeenAt.After(sessions[j].LastSeenAt)
	})
	return sessions, nil
}

func (s *RedisSessionStore) RotateSessionTokens(ctx context.Context, input RotateSessionTokensInput) (*Session, error) {
	watcher, ok := s.client.(interface {
		Watch(context.Context, func(*goredis.Tx) error, ...string) error
	})
	if !ok {
		return nil, fmt.Errorf("jwt redis client does not support watch")
	}
	var rotated *Session
	sessionKey := s.sessionKey(input.SID)
	err := watcher.Watch(ctx, func(tx *goredis.Tx) error {
		data, err := tx.Get(ctx, sessionKey).Bytes()
		if err == goredis.Nil {
			return ErrSessionNotFound
		}
		if err != nil {
			return fmt.Errorf("get jwt session failed: %w", err)
		}
		var session Session
		if err := json.Unmarshal(data, &session); err != nil {
			return fmt.Errorf("unmarshal jwt session failed: %w", err)
		}
		if session.Status != SessionStatusActive {
			return ErrSessionRevoked
		}
		if session.CurrentRefreshID != input.ExpectedRefresh {
			return ErrRefreshTokenReused
		}
		oldAccessID := session.CurrentAccessID
		oldRefreshID := session.CurrentRefreshID
		session.CurrentAccessID = input.NextAccess
		session.CurrentRefreshID = input.NextRefresh
		session.ExpiresAt = input.AccessExpiresAt
		session.RefreshExpiresAt = input.RefreshExpiresAt
		session.RefreshRotatedAt = input.Now
		session.LastSeenAt = input.Now
		payload, err := json.Marshal(session)
		if err != nil {
			return fmt.Errorf("marshal jwt session failed: %w", err)
		}
		ttl := time.Until(input.RefreshExpiresAt)
		if ttl <= 0 {
			return ErrTokenExpired
		}
		pipe := tx.TxPipeline()
		pipe.Set(ctx, sessionKey, payload, ttl)
		pipe.Del(ctx, s.accessKey(oldAccessID), s.refreshKey(oldRefreshID))
		pipe.Set(ctx, s.accessKey(input.NextAccess), input.SID, time.Until(input.AccessExpiresAt))
		pipe.Set(ctx, s.refreshKey(input.NextRefresh), input.SID, ttl)
		pipe.ZAdd(ctx, s.userSessionsKey(session.Subject), goredis.Z{Score: float64(input.Now.UnixMilli()), Member: input.SID})
		if _, err := pipe.Exec(ctx); err != nil {
			return fmt.Errorf("rotate jwt session tokens failed: %w", err)
		}
		rotated = cloneSession(session)
		return nil
	}, sessionKey)
	if err != nil {
		return nil, err
	}
	return rotated, nil
}

func (s *RedisSessionStore) TouchSession(ctx context.Context, sid string, now time.Time, interval time.Duration) error {
	session, err := s.GetSession(ctx, sid)
	if err == ErrSessionNotFound {
		return nil
	}
	if err != nil {
		return err
	}
	if session.Status != SessionStatusActive || interval > 0 && now.Sub(session.LastSeenAt) < interval {
		return nil
	}
	session.LastSeenAt = now
	if err := s.saveSession(ctx, *session); err != nil {
		return err
	}
	return s.client.ZAdd(ctx, s.userSessionsKey(session.Subject), goredis.Z{Score: float64(now.UnixMilli()), Member: sid}).Err()
}

func (s *RedisSessionStore) RevokeSession(ctx context.Context, sid string, reason string, now time.Time) error {
	session, err := s.GetSession(ctx, sid)
	if err != nil {
		return err
	}
	if err := s.deleteSession(ctx, *session); err != nil {
		return err
	}
	s.logger.InfoCtx(ctx, "jwt session deleted", zap.String("sid", sid), zap.String("reason", reason))
	return nil
}

func (s *RedisSessionStore) RevokeSubjectSessions(ctx context.Context, subject string, reason string, now time.Time) error {
	sids, err := s.client.ZRange(ctx, s.userSessionsKey(subject), 0, -1).Result()
	if err != nil {
		return fmt.Errorf("list jwt subject sessions failed: %w", err)
	}
	for _, sid := range sids {
		session, err := s.GetSession(ctx, sid)
		if err == ErrSessionNotFound {
			continue
		}
		if err != nil {
			return err
		}
		if err := s.deleteSession(ctx, *session); err != nil {
			return err
		}
	}
	if err := s.client.Del(ctx, s.userSessionsKey(subject)).Err(); err != nil {
		return fmt.Errorf("delete jwt subject sessions index failed: %w", err)
	}
	s.logger.InfoCtx(ctx, "jwt subject sessions deleted", zap.String("subject", subject), zap.String("reason", reason))
	return nil
}

func (s *RedisSessionStore) DeleteExpired(ctx context.Context, now time.Time) error {
	return nil
}

func (s *RedisSessionStore) Close() error {
	return nil
}

func (s *RedisSessionStore) saveSession(ctx context.Context, session Session) error {
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshal jwt session failed: %w", err)
	}
	ttl := time.Until(session.RefreshExpiresAt)
	if ttl <= 0 {
		return ErrTokenExpired
	}
	return s.client.Set(ctx, s.sessionKey(session.SID), data, ttl).Err()
}

func (s *RedisSessionStore) deleteSession(ctx context.Context, session Session) error {
	pipe := s.client.TxPipeline()
	pipe.Del(ctx, s.sessionKey(session.SID))
	tokenKeys := make([]string, 0, 2)
	if session.CurrentAccessID != "" {
		tokenKeys = append(tokenKeys, s.accessKey(session.CurrentAccessID))
	}
	if session.CurrentRefreshID != "" {
		tokenKeys = append(tokenKeys, s.refreshKey(session.CurrentRefreshID))
	}
	if len(tokenKeys) > 0 {
		pipe.Del(ctx, tokenKeys...)
	}
	pipe.ZRem(ctx, s.userSessionsKey(session.Subject), session.SID)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("delete jwt session failed: %w", err)
	}
	return nil
}

func (s *RedisSessionStore) sessionKey(sid string) string {
	return s.keyPrefix + "session:" + sid
}

func (s *RedisSessionStore) userSessionsKey(subject string) string {
	return s.keyPrefix + "user_sessions:" + subject
}

func (s *RedisSessionStore) accessKey(ati string) string {
	return s.keyPrefix + "access:" + ati
}

func (s *RedisSessionStore) refreshKey(rti string) string {
	return s.keyPrefix + "refresh:" + rti
}
