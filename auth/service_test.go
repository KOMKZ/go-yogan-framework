package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/stretchr/testify/assert"
)

// mock auth provider
type mockAuthProvider struct {
	name        string
	authErr     error
	authResult  *AuthResult
}

func (m *mockAuthProvider) Name() string {
	return m.name
}

func (m *mockAuthProvider) Authenticate(ctx context.Context, credentials Credentials) (*AuthResult, error) {
	if m.authErr != nil {
		return nil, m.authErr
	}
	return m.authResult, nil
}

func getTestLogger() *logger.CtxZapLogger {
	mgr := logger.NewManager(logger.ManagerConfig{
		Level:         "debug",
		Encoding:      "console",
		EnableConsole: true,
	})
	return mgr.GetLogger("test")
}

func TestNewAuthService(t *testing.T) {
	log := getTestLogger()
	svc := NewAuthService(log)

	assert.NotNil(t, svc)
	assert.NotNil(t, svc.providers)
	assert.Equal(t, log, svc.logger)
}

func TestAuthService_RegisterProvider(t *testing.T) {
	log := getTestLogger()
	svc := NewAuthService(log)

	provider := &mockAuthProvider{name: "password"}
	svc.RegisterProvider(provider)

	registered, ok := svc.GetProvider("password")
	assert.True(t, ok)
	assert.Equal(t, provider, registered)
}

func TestAuthService_GetProvider(t *testing.T) {
	log := getTestLogger()
	svc := NewAuthService(log)

	t.Run("provider exists", func(t *testing.T) {
		provider := &mockAuthProvider{name: "password"}
		svc.RegisterProvider(provider)

		p, ok := svc.GetProvider("password")
		assert.True(t, ok)
		assert.Equal(t, provider, p)
	})

	t.Run("provider not exists", func(t *testing.T) {
		p, ok := svc.GetProvider("unknown")
		assert.False(t, ok)
		assert.Nil(t, p)
	})
}

func TestAuthService_Authenticate(t *testing.T) {
	log := getTestLogger()
	ctx := context.Background()

	t.Run("provider not found", func(t *testing.T) {
		svc := NewAuthService(log)

		result, err := svc.Authenticate(ctx, "unknown", Credentials{})
		assert.Nil(t, result)
		assert.ErrorIs(t, err, ErrProviderNotSupported)
	})

	t.Run("authentication success", func(t *testing.T) {
		svc := NewAuthService(log)
		provider := &mockAuthProvider{
			name: "password",
			authResult: &AuthResult{
				UserID:   1,
				Username: "testuser",
				Email:    "test@example.com",
				Roles:    []string{"admin"},
			},
		}
		svc.RegisterProvider(provider)

		result, err := svc.Authenticate(ctx, "password", Credentials{
			Username: "testuser",
			Password: "password123",
		})

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, int64(1), result.UserID)
		assert.Equal(t, "testuser", result.Username)
	})

	t.Run("authentication failure", func(t *testing.T) {
		svc := NewAuthService(log)
		provider := &mockAuthProvider{
			name:    "password",
			authErr: errors.New("authentication failed"),
		}
		svc.RegisterProvider(provider)

		result, err := svc.Authenticate(ctx, "password", Credentials{
			Username: "testuser",
			Password: "wrongpassword",
		})

		assert.Error(t, err)
		assert.Nil(t, result)
	})
}
