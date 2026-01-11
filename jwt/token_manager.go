package jwt

import (
	"context"
	"fmt"
	"time"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// TokenManager Token 管理接口
type TokenManager interface {
	// GenerateAccessToken 生成 Access Token
	GenerateAccessToken(ctx context.Context, subject string, claims map[string]interface{}) (string, error)

	// GenerateRefreshToken 生成 Refresh Token
	GenerateRefreshToken(ctx context.Context, subject string) (string, error)

	// VerifyToken 验证并解析 Token
	VerifyToken(ctx context.Context, token string) (*Claims, error)

	// RefreshToken 刷新 Token（使用 Refresh Token 获取新的 Access Token）
	RefreshToken(ctx context.Context, refreshToken string) (string, error)

	// RevokeToken 撤销 Token（加入黑名单）
	RevokeToken(ctx context.Context, token string) error

	// RevokeUserTokens 批量撤销（用户登出所有设备）
	RevokeUserTokens(ctx context.Context, subject string) error
}

// tokenManagerImpl TokenManager 实现
type tokenManagerImpl struct {
	config        *Config
	signingMethod jwt.SigningMethod
	signingKey    interface{}
	verifyKey     interface{}
	tokenStore    TokenStore
	logger        *logger.CtxZapLogger
}

// NewTokenManager 创建 TokenManager
func NewTokenManager(config *Config, tokenStore TokenStore, log *logger.CtxZapLogger) (TokenManager, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	manager := &tokenManagerImpl{
		config:     config,
		tokenStore: tokenStore,
		logger:     log,
	}

	// 设置签名方法和密钥
	if err := manager.setupSigningMethod(); err != nil {
		return nil, err
	}

	return manager, nil
}

// setupSigningMethod 设置签名方法
func (m *tokenManagerImpl) setupSigningMethod() error {
	switch m.config.Algorithm {
	case "HS256":
		m.signingMethod = jwt.SigningMethodHS256
		m.signingKey = []byte(m.config.Secret)
		m.verifyKey = m.signingKey
	case "HS384":
		m.signingMethod = jwt.SigningMethodHS384
		m.signingKey = []byte(m.config.Secret)
		m.verifyKey = m.signingKey
	case "HS512":
		m.signingMethod = jwt.SigningMethodHS512
		m.signingKey = []byte(m.config.Secret)
		m.verifyKey = m.signingKey
	case "RS256":
		m.signingMethod = jwt.SigningMethodRS256
		// TODO: 加载 RSA 私钥和公钥
		return fmt.Errorf("RS256 not yet implemented")
	default:
		return ErrAlgorithmNotSupported
	}

	return nil
}

// GenerateAccessToken 生成 Access Token
func (m *tokenManagerImpl) GenerateAccessToken(ctx context.Context, subject string, customClaims map[string]interface{}) (string, error) {
	now := time.Now()
	expiresAt := now.Add(m.config.AccessToken.TTL)

	// 构建 Claims
	claims := jwt.MapClaims{
		"sub":        subject,
		"iat":        now.Unix(),
		"exp":        expiresAt.Unix(),
		"iss":        m.config.AccessToken.Issuer,
		"token_type": "access",
	}

	// 添加 Audience
	if m.config.AccessToken.Audience != "" {
		claims["aud"] = m.config.AccessToken.Audience
	}

	// 添加 JTI（防重放）
	if m.config.Security.EnableJTI {
		claims["jti"] = uuid.New().String()
	}

	// 添加 NotBefore
	if m.config.Security.EnableNotBefore {
		claims["nbf"] = now.Unix()
	}

	// 合并自定义 Claims
	for k, v := range customClaims {
		claims[k] = v
	}

	// 创建 Token
	token := jwt.NewWithClaims(m.signingMethod, claims)

	// 签名
	tokenString, err := token.SignedString(m.signingKey)
	if err != nil {
		m.logger.ErrorCtx(ctx, "failed to sign token",
			zap.Error(err),
			zap.String("subject", subject),
		)
		return "", fmt.Errorf("sign token failed: %w", err)
	}

	m.logger.DebugCtx(ctx, "access token generated",
		zap.String("subject", subject),
		zap.Duration("ttl", m.config.AccessToken.TTL),
	)

	return tokenString, nil
}

// GenerateRefreshToken 生成 Refresh Token
func (m *tokenManagerImpl) GenerateRefreshToken(ctx context.Context, subject string) (string, error) {
	if !m.config.RefreshToken.Enabled {
		return "", fmt.Errorf("refresh token not enabled")
	}

	now := time.Now()
	expiresAt := now.Add(m.config.RefreshToken.TTL)

	claims := jwt.MapClaims{
		"sub":        subject,
		"iat":        now.Unix(),
		"exp":        expiresAt.Unix(),
		"token_type": "refresh",
		"jti":        uuid.New().String(),
	}

	token := jwt.NewWithClaims(m.signingMethod, claims)
	tokenString, err := token.SignedString(m.signingKey)
	if err != nil {
		m.logger.ErrorCtx(ctx, "failed to sign refresh token",
			zap.Error(err),
			zap.String("subject", subject),
		)
		return "", fmt.Errorf("sign refresh token failed: %w", err)
	}

	m.logger.DebugCtx(ctx, "refresh token generated",
		zap.String("subject", subject),
		zap.Duration("ttl", m.config.RefreshToken.TTL),
	)

	return tokenString, nil
}

// VerifyToken 验证并解析 Token
func (m *tokenManagerImpl) VerifyToken(ctx context.Context, tokenString string) (*Claims, error) {
	// 解析 Token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// 验证签名算法
		if token.Method != m.signingMethod {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return m.verifyKey, nil
	})

	if err != nil {
		m.logger.WarnCtx(ctx, "token verification failed",
			zap.Error(err),
		)
		return nil, m.parseJWTError(err)
	}

	if !token.Valid {
		return nil, ErrTokenInvalid
	}

	// 提取 Claims
	mapClaims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrInvalidClaims
	}

	// 转换为自定义 Claims
	claims, err := m.parseCustomClaims(mapClaims)
	if err != nil {
		m.logger.WarnCtx(ctx, "failed to parse claims",
			zap.Error(err),
		)
		return nil, err
	}

	// 检查黑名单
	if m.config.Blacklist.Enabled && m.tokenStore != nil {
		// 检查 Token 黑名单
		blacklisted, err := m.tokenStore.IsBlacklisted(ctx, tokenString)
		if err != nil {
			m.logger.ErrorCtx(ctx, "failed to check token blacklist",
				zap.Error(err),
			)
			return nil, fmt.Errorf("check blacklist failed: %w", err)
		}
		if blacklisted {
			m.logger.WarnCtx(ctx, "token is blacklisted",
				zap.String("subject", claims.Subject),
			)
			return nil, ErrTokenBlacklisted
		}

		// 检查用户黑名单
		userBlacklisted, err := m.tokenStore.IsUserBlacklisted(ctx, claims.Subject, claims.IssuedAt)
		if err != nil {
			m.logger.ErrorCtx(ctx, "failed to check user blacklist",
				zap.Error(err),
			)
			return nil, fmt.Errorf("check user blacklist failed: %w", err)
		}
		if userBlacklisted {
			m.logger.WarnCtx(ctx, "user is blacklisted",
				zap.String("subject", claims.Subject),
			)
			return nil, ErrTokenBlacklisted
		}
	}

	m.logger.DebugCtx(ctx, "token verified",
		zap.String("subject", claims.Subject),
	)

	return claims, nil
}

// RefreshToken 刷新 Token
func (m *tokenManagerImpl) RefreshToken(ctx context.Context, refreshToken string) (string, error) {
	// 验证 Refresh Token
	claims, err := m.VerifyToken(ctx, refreshToken)
	if err != nil {
		return "", fmt.Errorf("invalid refresh token: %w", err)
	}

	// 检查 Token 类型
	if claims.TokenType != "refresh" {
		return "", fmt.Errorf("not a refresh token")
	}

	// 生成新的 Access Token
	customClaims := make(map[string]interface{})
	if claims.UserID > 0 {
		customClaims["user_id"] = claims.UserID
	}
	if claims.Username != "" {
		customClaims["username"] = claims.Username
	}
	if len(claims.Roles) > 0 {
		customClaims["roles"] = claims.Roles
	}
	if claims.TenantID != "" {
		customClaims["tenant_id"] = claims.TenantID
	}

	accessToken, err := m.GenerateAccessToken(ctx, claims.Subject, customClaims)
	if err != nil {
		return "", fmt.Errorf("generate access token failed: %w", err)
	}

	m.logger.InfoCtx(ctx, "token refreshed",
		zap.String("subject", claims.Subject),
	)

	return accessToken, nil
}

// RevokeToken 撤销 Token
func (m *tokenManagerImpl) RevokeToken(ctx context.Context, tokenString string) error {
	if !m.config.Blacklist.Enabled || m.tokenStore == nil {
		return fmt.Errorf("blacklist not enabled")
	}

	// 解析 Token 获取过期时间
	claims, err := m.VerifyToken(ctx, tokenString)
	if err != nil {
		// Token 已失效，无需加入黑名单
		return nil
	}

	// 添加到黑名单，TTL 为剩余过期时间
	ttl := claims.TTL()
	if ttl <= 0 {
		return nil // 已过期
	}

	err = m.tokenStore.AddToBlacklist(ctx, tokenString, ttl)
	if err != nil {
		return fmt.Errorf("add to blacklist failed: %w", err)
	}

	m.logger.InfoCtx(ctx, "token revoked",
		zap.String("subject", claims.Subject),
		zap.Duration("ttl", ttl),
	)

	return nil
}

// RevokeUserTokens 撤销用户所有 Token
func (m *tokenManagerImpl) RevokeUserTokens(ctx context.Context, subject string) error {
	if !m.config.Blacklist.Enabled || m.tokenStore == nil {
		return fmt.Errorf("blacklist not enabled")
	}

	// 使用 Access Token TTL 作为黑名单 TTL
	ttl := m.config.AccessToken.TTL

	err := m.tokenStore.BlacklistUserTokens(ctx, subject, ttl)
	if err != nil {
		return fmt.Errorf("blacklist user tokens failed: %w", err)
	}

	m.logger.InfoCtx(ctx, "user tokens revoked",
		zap.String("subject", subject),
	)

	return nil
}

// parseCustomClaims 解析自定义 Claims
func (m *tokenManagerImpl) parseCustomClaims(mapClaims jwt.MapClaims) (*Claims, error) {
	claims := &Claims{}

	// 标准 Claims
	if sub, ok := mapClaims["sub"].(string); ok {
		claims.Subject = sub
	}

	if iat, ok := mapClaims["iat"].(float64); ok {
		claims.IssuedAt = time.Unix(int64(iat), 0)
	}

	if exp, ok := mapClaims["exp"].(float64); ok {
		claims.ExpiresAt = time.Unix(int64(exp), 0)
	}

	if nbf, ok := mapClaims["nbf"].(float64); ok {
		claims.NotBefore = time.Unix(int64(nbf), 0)
	}

	if iss, ok := mapClaims["iss"].(string); ok {
		claims.Issuer = iss
	}

	if aud, ok := mapClaims["aud"].(string); ok {
		claims.Audience = aud
	}

	if jti, ok := mapClaims["jti"].(string); ok {
		claims.JTI = jti
	}

	if tokenType, ok := mapClaims["token_type"].(string); ok {
		claims.TokenType = tokenType
	}

	// 自定义 Claims
	if userID, ok := mapClaims["user_id"].(float64); ok {
		claims.UserID = int64(userID)
	}

	if username, ok := mapClaims["username"].(string); ok {
		claims.Username = username
	}

	if roles, ok := mapClaims["roles"].([]interface{}); ok {
		for _, role := range roles {
			if r, ok := role.(string); ok {
				claims.Roles = append(claims.Roles, r)
			}
		}
	}

	if tenantID, ok := mapClaims["tenant_id"].(string); ok {
		claims.TenantID = tenantID
	}

	// 验证 Claims
	if err := claims.Valid(); err != nil {
		return nil, err
	}

	return claims, nil
}

// parseJWTError 解析 JWT 错误
func (m *tokenManagerImpl) parseJWTError(err error) error {
	// 使用 errors.Is 来检查错误链
	if err == nil {
		return nil
	}

	// 检查具体错误类型
	switch {
	case err == jwt.ErrTokenExpired:
		return ErrTokenExpired
	case err == jwt.ErrTokenNotValidYet:
		return ErrTokenNotYetValid
	case err == jwt.ErrTokenSignatureInvalid:
		return ErrInvalidSignature
	}

	// 检查错误字符串（golang-jwt/jwt/v5 的错误可能被包装）
	errStr := err.Error()
	switch {
	case contains(errStr, "expired"):
		return ErrTokenExpired
	case contains(errStr, "not valid yet"):
		return ErrTokenNotYetValid
	case contains(errStr, "signature"):
		return ErrInvalidSignature
	default:
		return ErrTokenInvalid
	}
}

// contains 检查字符串是否包含子串
func contains(s, substr string) bool {
	if substr == "" {
		return true
	}
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
