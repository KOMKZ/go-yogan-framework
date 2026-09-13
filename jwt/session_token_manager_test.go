package jwt

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/stretchr/testify/require"
)

func newSessionTestManager(t *testing.T, maxSessions int) SessionTokenManager {
	t.Helper()
	cfg := newTestConfig()
	cfg.Session = SessionConfig{
		Enabled:                true,
		Store:                  "memory",
		KeyPrefix:              "jwt:test:",
		MaxSessionsPerSubject:  maxSessions,
		LastSeenUpdateInterval: time.Minute,
		RefreshReusePolicy:     SessionRefreshReuseRevokeSession,
		CleanupInterval:        time.Hour,
	}
	log := logger.NewCtxZapLogger("yogan")
	store := NewMemorySessionStore(cfg.Session, log)
	t.Cleanup(func() { _ = store.Close() })
	manager, err := NewTokenManager(cfg, store, log)
	require.NoError(t, err)
	sessionManager, ok := manager.(SessionTokenManager)
	require.True(t, ok)
	return sessionManager
}

func TestSessionTokenManager_IssueAndVerify(t *testing.T) {
	manager := newSessionTestManager(t, 3)
	pair, err := manager.IssueTokenPair(context.Background(), IssueTokenInput{
		Subject: "admin-1",
		Claims:  map[string]interface{}{"user_id": int64(1), "username": "root"},
		Client:  ClientInfo{IP: "127.0.0.1", UserAgent: "test"},
	})
	require.NoError(t, err)
	require.NotEmpty(t, pair.Session.SID)

	claims, err := manager.VerifyToken(context.Background(), pair.AccessToken)
	require.NoError(t, err)
	require.Equal(t, pair.Session.SID, claims.SID)
	require.Equal(t, pair.Session.CurrentAccessID, claims.ATI)
	require.Equal(t, int64(1), claims.UserID)
}

func TestSessionTokenManager_RevokeSession(t *testing.T) {
	manager := newSessionTestManager(t, 3)
	pair, err := manager.IssueTokenPair(context.Background(), IssueTokenInput{Subject: "admin-1"})
	require.NoError(t, err)

	require.NoError(t, manager.RevokeSession(context.Background(), pair.Session.SID, "device_logout"))
	_, err = manager.VerifyToken(context.Background(), pair.AccessToken)
	require.ErrorIs(t, err, ErrSessionNotFound)
}

func TestSessionTokenManager_RevokeSubjectSessions(t *testing.T) {
	manager := newSessionTestManager(t, 3)
	first, err := manager.IssueTokenPair(context.Background(), IssueTokenInput{Subject: "admin-1"})
	require.NoError(t, err)
	second, err := manager.IssueTokenPair(context.Background(), IssueTokenInput{Subject: "admin-1"})
	require.NoError(t, err)

	require.NoError(t, manager.RevokeSubjectSessions(context.Background(), "admin-1", "permission_changed"))
	_, err = manager.VerifyToken(context.Background(), first.AccessToken)
	require.ErrorIs(t, err, ErrSessionNotFound)
	_, err = manager.VerifyToken(context.Background(), second.AccessToken)
	require.ErrorIs(t, err, ErrSessionNotFound)
}

func TestSessionTokenManager_MaxSessions(t *testing.T) {
	manager := newSessionTestManager(t, 1)
	first, err := manager.IssueTokenPair(context.Background(), IssueTokenInput{Subject: "admin-1"})
	require.NoError(t, err)
	second, err := manager.IssueTokenPair(context.Background(), IssueTokenInput{Subject: "admin-1"})
	require.NoError(t, err)

	_, err = manager.VerifyToken(context.Background(), first.AccessToken)
	require.ErrorIs(t, err, ErrSessionNotFound)
	_, err = manager.VerifyToken(context.Background(), second.AccessToken)
	require.NoError(t, err)
}

func TestSessionTokenManager_MaxThreeSessionsEvictsOldest(t *testing.T) {
	manager := newSessionTestManager(t, 3)
	first, err := manager.IssueTokenPair(context.Background(), IssueTokenInput{Subject: "admin-1"})
	require.NoError(t, err)
	time.Sleep(2 * time.Millisecond)
	second, err := manager.IssueTokenPair(context.Background(), IssueTokenInput{Subject: "admin-1"})
	require.NoError(t, err)
	time.Sleep(2 * time.Millisecond)
	third, err := manager.IssueTokenPair(context.Background(), IssueTokenInput{Subject: "admin-1"})
	require.NoError(t, err)
	time.Sleep(2 * time.Millisecond)
	fourth, err := manager.IssueTokenPair(context.Background(), IssueTokenInput{Subject: "admin-1"})
	require.NoError(t, err)

	_, err = manager.VerifyToken(context.Background(), first.AccessToken)
	require.ErrorIs(t, err, ErrSessionNotFound)
	for _, pair := range []*TokenPair{second, third, fourth} {
		_, err = manager.VerifyToken(context.Background(), pair.AccessToken)
		require.NoError(t, err)
	}
	sessions, err := manager.ListSubjectSessions(context.Background(), "admin-1")
	require.NoError(t, err)
	require.Len(t, sessions, 3)
}

func TestSessionTokenManager_CompactsStoredSessionClaims(t *testing.T) {
	manager := newSessionTestManager(t, 3)
	pair, err := manager.IssueTokenPair(context.Background(), IssueTokenInput{
		Subject: "admin-1",
		Claims: map[string]interface{}{
			"user_id":  int64(1),
			"username": "root",
			"email":    "root@example.test",
			"roles":    []string{"admin", "ops"},
			"profile":  map[string]string{"large": "payload"},
		},
	})
	require.NoError(t, err)

	require.Equal(t, map[string]interface{}{
		"user_id":  int64(1),
		"username": "root",
		"email":    "root@example.test",
	}, pair.Session.Claims)
}

func TestSessionTokenManager_RefreshReuseRevokesSession(t *testing.T) {
	manager := newSessionTestManager(t, 3)
	pair, err := manager.IssueTokenPair(context.Background(), IssueTokenInput{Subject: "admin-1"})
	require.NoError(t, err)

	rotated, err := manager.RefreshTokenPair(context.Background(), RefreshTokenInput{RefreshToken: pair.RefreshToken})
	require.NoError(t, err)
	require.NotEqual(t, pair.Session.CurrentRefreshID, rotated.Session.CurrentRefreshID)

	_, err = manager.RefreshTokenPair(context.Background(), RefreshTokenInput{RefreshToken: pair.RefreshToken})
	require.True(t, errors.Is(err, ErrRefreshTokenReused) || errors.Is(err, ErrSessionRevoked))
	_, err = manager.VerifyToken(context.Background(), rotated.AccessToken)
	require.ErrorIs(t, err, ErrSessionNotFound)
}
