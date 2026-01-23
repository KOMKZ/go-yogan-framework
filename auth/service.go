package auth

import (
	"context"
	"time"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"go.uber.org/zap"
)

// Authentication service
type AuthService struct {
	providers map[string]AuthProvider
	logger    *logger.CtxZapLogger
	metrics   *AuthMetrics // Optional: metrics provider (injected after creation)
}

// Create authentication service
func NewAuthService(logger *logger.CtxZapLogger) *AuthService {
	return &AuthService{
		providers: make(map[string]AuthProvider),
		logger:    logger,
	}
}

// SetMetrics injects the Auth metrics provider.
// This should be called after the AuthService is created when metrics are enabled.
func (s *AuthService) SetMetrics(metrics *AuthMetrics) {
	s.metrics = metrics
}

// RegisterAuthProvider:Register the authentication provider
func (s *AuthService) RegisterProvider(provider AuthProvider) {
	s.providers[provider.Name()] = provider
	s.logger.InfoCtx(context.Background(), "auth provider registered",
		zap.String("provider", provider.Name()))
}

// Authenticate execution authorization
func (s *AuthService) Authenticate(ctx context.Context, providerName string, credentials Credentials) (*AuthResult, error) {
	start := time.Now()

	// 1. Get the authentication provider
	provider, ok := s.providers[providerName]
	if !ok {
		// Record Metrics: provider not found
		if s.metrics != nil {
			s.metrics.RecordLogin(ctx, providerName, "provider_not_found", time.Since(start))
		}
		return nil, ErrProviderNotSupported
	}

	// 2. Perform authentication
	result, err := provider.Authenticate(ctx, credentials)
	if err != nil {
		s.logger.WarnCtx(ctx, "authentication failed",
			zap.String("provider", providerName),
			zap.String("username", credentials.Username),
			zap.Error(err))

		// Record Metrics: authentication failed
		if s.metrics != nil {
			s.metrics.RecordLogin(ctx, providerName, "failed", time.Since(start))
			s.metrics.RecordFailedAttempt(ctx, err.Error())
		}
		return nil, err
	}

	// Record successful authentication log
	s.logger.InfoCtx(ctx, "authentication successful",
		zap.String("provider", providerName),
		zap.Int64("user_id", result.UserID),
		zap.String("username", result.Username))

	// Record Metrics: authentication success
	if s.metrics != nil {
		s.metrics.RecordLogin(ctx, providerName, "success", time.Since(start))
	}

	return result, nil
}

// GetProvider Get authentication provider
func (s *AuthService) GetProvider(name string) (AuthProvider, bool) {
	provider, ok := s.providers[name]
	return provider, ok
}

