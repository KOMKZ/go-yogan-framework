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

// Token Manager token management interface
type TokenManager interface {
	// GenerateAccessToken generates Access Token
	GenerateAccessToken(ctx context.Context, subject string, claims map[string]interface{}) (string, error)

	// GenerateRefreshToken generates Refresh Token
	GenerateRefreshToken(ctx context.Context, subject string) (string, error)

	// VerifyToken validate and parse the token
	VerifyToken(ctx context.Context, token string) (*Claims, error)

	// RefreshToken refresh token (use refresh token to obtain a new access token)
	RefreshToken(ctx context.Context, refreshToken string) (string, error)

	// RevokeToken Revoke Token (Add to blacklist)
	RevokeToken(ctx context.Context, token string) error

	// RevokeUserTokens batch revoke user tokens (log out user from all devices)
	RevokeUserTokens(ctx context.Context, subject string) error
}

// tokenManagerImpl TokenManager implementation
type tokenManagerImpl struct {
	config        *Config
	signingMethod jwt.SigningMethod
	signingKey    interface{}
	verifyKey     interface{}
	tokenStore    TokenStore
	logger        *logger.CtxZapLogger
}

// NewTokenManager creates TokenManager
func NewTokenManager(config *Config, tokenStore TokenStore, log *logger.CtxZapLogger) (TokenManager, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	manager := &tokenManagerImpl{
		config:     config,
		tokenStore: tokenStore,
		logger:     log,
	}

	// Set signature method and key
	if err := manager.setupSigningMethod(); err != nil {
		return nil, err
	}

	return manager, nil
}

// set up signing method
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
		// TODO: Load RSA private key and public key
		return fmt.Errorf("RS256 not yet implemented")
	default:
		return ErrAlgorithmNotSupported
	}

	return nil
}

// GenerateAccessToken generates Access Token
func (m *tokenManagerImpl) GenerateAccessToken(ctx context.Context, subject string, customClaims map[string]interface{}) (string, error) {
	now := time.Now()
	expiresAt := now.Add(m.config.AccessToken.TTL)

	// Construct Claims
	claims := jwt.MapClaims{
		"sub":        subject,
		"iat":        now.Unix(),
		"exp":        expiresAt.Unix(),
		"iss":        m.config.AccessToken.Issuer,
		"token_type": "access",
	}

	// Add Audience
	if m.config.AccessToken.Audience != "" {
		claims["aud"] = m.config.AccessToken.Audience
	}

	// Add JTI (anti-replay)
	if m.config.Security.EnableJTI {
		claims["jti"] = uuid.New().String()
	}

	// Add NotBefore
	if m.config.Security.EnableNotBefore {
		claims["nbf"] = now.Unix()
	}

	// Merge custom claims
	for k, v := range customClaims {
		claims[k] = v
	}

	// Create Token
	token := jwt.NewWithClaims(m.signingMethod, claims)

	// Signature
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

// GenerateRefreshToken generates Refresh Token
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

// VerifyToken validate and parse Token
func (m *tokenManagerImpl) VerifyToken(ctx context.Context, tokenString string) (*Claims, error) {
	// Parse Token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Verify signature algorithm
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

	// Extract claims
	mapClaims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrInvalidClaims
	}

	// Convert to custom claims
	claims, err := m.parseCustomClaims(mapClaims)
	if err != nil {
		m.logger.WarnCtx(ctx, "failed to parse claims",
			zap.Error(err),
		)
		return nil, err
	}

	// Check blacklist
	if m.config.Blacklist.Enabled && m.tokenStore != nil {
		// Check Token blacklist
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

		// Check user blacklist
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

// RefreshToken refresh token
func (m *tokenManagerImpl) RefreshToken(ctx context.Context, refreshToken string) (string, error) {
	// Validate Refresh Token
	claims, err := m.VerifyToken(ctx, refreshToken)
	if err != nil {
		return "", fmt.Errorf("invalid refresh token: %w", err)
	}

	// Check token type
	if claims.TokenType != "refresh" {
		return "", fmt.Errorf("not a refresh token")
	}

	// Generate new Access Token
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

// RevokeToken revoke token
func (m *tokenManagerImpl) RevokeToken(ctx context.Context, tokenString string) error {
	if !m.config.Blacklist.Enabled || m.tokenStore == nil {
		return fmt.Errorf("blacklist not enabled")
	}

	// Parse token to get expiration time
	claims, err := m.VerifyToken(ctx, tokenString)
	if err != nil {
		// Token has expired, no need to add to blacklist
		return nil
	}

	// Add to blacklist, TTL is remaining expiry time
	ttl := claims.TTL()
	if ttl <= 0 {
		return nil // expired
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

// RevokeUserTokens Revoke all user tokens
func (m *tokenManagerImpl) RevokeUserTokens(ctx context.Context, subject string) error {
	if !m.config.Blacklist.Enabled || m.tokenStore == nil {
		return fmt.Errorf("blacklist not enabled")
	}

	// Use the Access Token TTL as the blacklist TTL
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

// parseCustomClaims Parse custom claims
func (m *tokenManagerImpl) parseCustomClaims(mapClaims jwt.MapClaims) (*Claims, error) {
	claims := &Claims{}

	// Standard Claims
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

	// Custom Claims
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

	// Validate Claims
	if err := claims.Valid(); err != nil {
		return nil, err
	}

	return claims, nil
}

// parseJWTError Parse JWT error
func (m *tokenManagerImpl) parseJWTError(err error) error {
	// Use errors.Is to check the error chain
	if err == nil {
		return nil
	}

	// Check specific error type
	switch {
	case err == jwt.ErrTokenExpired:
		return ErrTokenExpired
	case err == jwt.ErrTokenNotValidYet:
		return ErrTokenNotYetValid
	case err == jwt.ErrTokenSignatureInvalid:
		return ErrInvalidSignature
	}

	// Check for error strings (errors from golang-jwt/jwt/v5 may be wrapped)
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

// contains Check if the string contains a substring
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
