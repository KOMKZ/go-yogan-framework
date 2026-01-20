package auth

import (
	"context"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"go.uber.org/zap"
)

// Authentication service
type AuthService struct {
	providers map[string]AuthProvider
	logger    *logger.CtxZapLogger
}

// Create authentication service
func NewAuthService(logger *logger.CtxZapLogger) *AuthService {
	return &AuthService{
		providers: make(map[string]AuthProvider),
		logger:    logger,
	}
}

// RegisterAuthProvider:Register the authentication provider
func (s *AuthService) RegisterProvider(provider AuthProvider) {
	s.providers[provider.Name()] = provider
	s.logger.InfoCtx(context.Background(), "auth provider registered",
		zap.String("provider", provider.Name()))
}

// Authenticate execution authorization
func (s *AuthService) Authenticate(ctx context.Context, providerName string, credentials Credentials) (*AuthResult, error) {
	// 1. Get the authentication provider
	provider, ok := s.providers[providerName]
	if !ok {
		return nil, ErrProviderNotSupported
	}

	// 2. Perform authentication
	result, err := provider.Authenticate(ctx, credentials)
	if err != nil {
		s.logger.WarnCtx(ctx, "authentication failed",
			zap.String("provider", providerName),
			zap.String("username", credentials.Username),
			zap.Error(err))
		return nil, err
	}

	// Record successful authentication log
	s.logger.InfoCtx(ctx, "authentication successful",
		zap.String("provider", providerName),
		zap.Int64("user_id", result.UserID),
		zap.String("username", result.Username))

	return result, nil
}

// GetProvider Get authentication provider
func (s *AuthService) GetProvider(name string) (AuthProvider, bool) {
	provider, ok := s.providers[name]
	return provider, ok
}

