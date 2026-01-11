package auth

import (
	"context"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"go.uber.org/zap"
)

// AuthService 认证服务
type AuthService struct {
	providers map[string]AuthProvider
	logger    *logger.CtxZapLogger
}

// NewAuthService 创建认证服务
func NewAuthService(logger *logger.CtxZapLogger) *AuthService {
	return &AuthService{
		providers: make(map[string]AuthProvider),
		logger:    logger,
	}
}

// RegisterProvider 注册认证提供者
func (s *AuthService) RegisterProvider(provider AuthProvider) {
	s.providers[provider.Name()] = provider
	s.logger.InfoCtx(context.Background(), "auth provider registered",
		zap.String("provider", provider.Name()))
}

// Authenticate 执行认证
func (s *AuthService) Authenticate(ctx context.Context, providerName string, credentials Credentials) (*AuthResult, error) {
	// 1. 获取认证提供者
	provider, ok := s.providers[providerName]
	if !ok {
		return nil, ErrProviderNotSupported
	}

	// 2. 执行认证
	result, err := provider.Authenticate(ctx, credentials)
	if err != nil {
		s.logger.WarnCtx(ctx, "authentication failed",
			zap.String("provider", providerName),
			zap.String("username", credentials.Username),
			zap.Error(err))
		return nil, err
	}

	// 3. 记录认证成功日志
	s.logger.InfoCtx(ctx, "authentication successful",
		zap.String("provider", providerName),
		zap.Int64("user_id", result.UserID),
		zap.String("username", result.Username))

	return result, nil
}

// GetProvider 获取认证提供者
func (s *AuthService) GetProvider(name string) (AuthProvider, bool) {
	provider, ok := s.providers[name]
	return provider, ok
}

