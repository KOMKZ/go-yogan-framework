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

	// RevokeToken revokes the token's server-side session when present.
	RevokeToken(ctx context.Context, token string) error

	// RevokeUserTokens batch revoke user tokens (log out user from all devices)
	RevokeUserTokens(ctx context.Context, subject string) error
}

// SessionTokenManager exposes server-side session aware JWT operations.
type SessionTokenManager interface {
	TokenManager
	IssueTokenPair(ctx context.Context, input IssueTokenInput) (*TokenPair, error)
	RefreshTokenPair(ctx context.Context, input RefreshTokenInput) (*TokenPair, error)
	RevokeSession(ctx context.Context, sid string, reason string) error
	RevokeSubjectSessions(ctx context.Context, subject string, reason string) error
	ListSubjectSessions(ctx context.Context, subject string) ([]Session, error)
}

// tokenManagerImpl TokenManager implementation
type tokenManagerImpl struct {
	config        *Config
	signingMethod jwt.SigningMethod
	signingKey    interface{}
	verifyKey     interface{}
	session       *SessionManager
	logger        *logger.CtxZapLogger
	metrics       *JWTMetrics // Optional: metrics provider (injected after creation)
}

// NewTokenManager creates TokenManager
func NewTokenManager(config *Config, sessionStore interface{}, log *logger.CtxZapLogger) (TokenManager, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	manager := &tokenManagerImpl{
		config: config,
		logger: log,
	}
	if config.Session.Enabled {
		store, ok := sessionStore.(SessionStore)
		if !ok || store == nil {
			return nil, ErrSessionRequired
		}
		manager.session = NewSessionManager(config.Session, store)
	}

	// Set signature method and key
	if err := manager.setupSigningMethod(); err != nil {
		return nil, err
	}

	return manager, nil
}

// IssueTokenPair creates an access/refresh pair bound to a server-side session.
func (m *tokenManagerImpl) IssueTokenPair(ctx context.Context, input IssueTokenInput) (*TokenPair, error) {
	if input.Subject == "" {
		return nil, ErrInvalidClaims
	}
	if m.session == nil {
		accessToken, err := m.GenerateAccessToken(ctx, input.Subject, input.Claims)
		if err != nil {
			return nil, err
		}
		refreshToken, err := m.GenerateRefreshToken(ctx, input.Subject)
		if err != nil {
			return nil, err
		}
		return &TokenPair{AccessToken: accessToken, RefreshToken: refreshToken, ExpiresIn: int64(m.config.AccessToken.TTL.Seconds())}, nil
	}
	now := time.Now()
	session, err := m.session.Create(ctx, input, m.config.AccessToken.TTL, m.config.RefreshToken.TTL, now)
	if err != nil {
		return nil, err
	}
	accessToken, err := m.signAccessToken(ctx, input.Subject, session.SID, session.CurrentAccessID, input.Claims, now, session.ExpiresAt)
	if err != nil {
		return nil, err
	}
	refreshToken, err := m.signRefreshToken(ctx, input.Subject, session.SID, session.CurrentRefreshID, now, session.RefreshExpiresAt)
	if err != nil {
		return nil, err
	}
	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		Session:      session,
		ExpiresIn:    int64(m.config.AccessToken.TTL.Seconds()),
		RefreshAfter: int64((m.config.AccessToken.TTL / 2).Seconds()),
	}, nil
}

// RefreshTokenPair rotates a refresh token and returns a new access/refresh pair.
func (m *tokenManagerImpl) RefreshTokenPair(ctx context.Context, input RefreshTokenInput) (*TokenPair, error) {
	claims, err := m.parseSignedToken(ctx, input.RefreshToken)
	if err != nil {
		return nil, err
	}
	if claims.TokenType != "refresh" {
		return nil, fmt.Errorf("not a refresh token")
	}
	if m.session == nil {
		accessToken, err := m.RefreshToken(ctx, input.RefreshToken)
		if err != nil {
			return nil, err
		}
		return &TokenPair{AccessToken: accessToken, RefreshToken: input.RefreshToken, ExpiresIn: int64(m.config.AccessToken.TTL.Seconds())}, nil
	}
	now := time.Now()
	session, err := m.session.Rotate(ctx, claims, now, m.config.AccessToken.TTL, m.config.RefreshToken.TTL)
	if err != nil {
		return nil, err
	}
	accessClaims := cloneClaimsMap(session.Claims)
	accessToken, err := m.signAccessToken(ctx, claims.Subject, session.SID, session.CurrentAccessID, accessClaims, now, session.ExpiresAt)
	if err != nil {
		return nil, err
	}
	refreshToken, err := m.signRefreshToken(ctx, claims.Subject, session.SID, session.CurrentRefreshID, now, session.RefreshExpiresAt)
	if err != nil {
		return nil, err
	}
	return &TokenPair{AccessToken: accessToken, RefreshToken: refreshToken, Session: session, ExpiresIn: int64(m.config.AccessToken.TTL.Seconds()), RefreshAfter: int64((m.config.AccessToken.TTL / 2).Seconds())}, nil
}

// SetMetrics injects the JWT metrics provider.
// This should be called after the TokenManager is created when metrics are enabled.
func (m *tokenManagerImpl) SetMetrics(metrics *JWTMetrics) {
	m.metrics = metrics
}

// SetTokenManagerMetrics is a helper function to inject metrics into a TokenManager.
// Returns true if successful, false if the TokenManager doesn't support metrics injection.
func SetTokenManagerMetrics(mgr TokenManager, metrics *JWTMetrics) bool {
	if impl, ok := mgr.(*tokenManagerImpl); ok {
		impl.SetMetrics(metrics)
		return true
	}
	return false
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
	return m.signAccessToken(ctx, subject, "", "", customClaims, now, expiresAt)
}

func (m *tokenManagerImpl) signAccessToken(ctx context.Context, subject string, sid string, ati string, customClaims map[string]interface{}, now time.Time, expiresAt time.Time) (string, error) {

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
	if sid != "" {
		claims["sid"] = sid
	}
	if ati != "" {
		claims["ati"] = ati
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

	// Record Metrics
	if m.metrics != nil {
		m.metrics.RecordGenerated(ctx, "access")
	}

	return tokenString, nil
}

// GenerateRefreshToken generates Refresh Token
func (m *tokenManagerImpl) GenerateRefreshToken(ctx context.Context, subject string) (string, error) {
	if !m.config.RefreshToken.Enabled {
		return "", fmt.Errorf("refresh token not enabled")
	}

	now := time.Now()
	expiresAt := now.Add(m.config.RefreshToken.TTL)
	return m.signRefreshToken(ctx, subject, "", "", now, expiresAt)
}

func (m *tokenManagerImpl) signRefreshToken(ctx context.Context, subject string, sid string, rti string, now time.Time, expiresAt time.Time) (string, error) {
	claims := jwt.MapClaims{
		"sub":        subject,
		"iat":        now.Unix(),
		"exp":        expiresAt.Unix(),
		"token_type": "refresh",
		"jti":        uuid.New().String(),
	}
	if sid != "" {
		claims["sid"] = sid
	}
	if rti != "" {
		claims["rti"] = rti
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

	// Record Metrics
	if m.metrics != nil {
		m.metrics.RecordGenerated(ctx, "refresh")
	}

	return tokenString, nil
}

// VerifyToken validate and parse Token
func (m *tokenManagerImpl) VerifyToken(ctx context.Context, tokenString string) (*Claims, error) {
	start := time.Now()
	claims, err := m.parseSignedToken(ctx, tokenString)
	if err != nil {
		if m.metrics != nil {
			m.metrics.RecordVerified(ctx, "error", time.Since(start))
		}
		return nil, err
	}

	if m.session != nil {
		if err := m.session.Verify(ctx, claims, time.Now()); err != nil {
			if m.metrics != nil {
				m.metrics.RecordVerified(ctx, "error", time.Since(start))
			}
			return nil, err
		}
	}

	m.logger.DebugCtx(ctx, "token verified",
		zap.String("subject", claims.Subject),
	)

	// Record successful verification
	if m.metrics != nil {
		m.metrics.RecordVerified(ctx, "success", time.Since(start))
	}

	return claims, nil
}

func (m *tokenManagerImpl) parseSignedToken(ctx context.Context, tokenString string) (*Claims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if token.Method != m.signingMethod {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return m.verifyKey, nil
	}, jwt.WithLeeway(m.config.Security.ClockSkew))
	if err != nil {
		m.logger.WarnCtx(ctx, "token verification failed", zap.Error(err))
		return nil, m.parseJWTError(err)
	}
	if !token.Valid {
		return nil, ErrTokenInvalid
	}
	mapClaims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrInvalidClaims
	}
	claims, err := m.parseCustomClaims(mapClaims)
	if err != nil {
		m.logger.WarnCtx(ctx, "failed to parse claims", zap.Error(err))
		return nil, err
	}
	return claims, nil
}

// RefreshToken refresh token
func (m *tokenManagerImpl) RefreshToken(ctx context.Context, refreshToken string) (string, error) {
	if m.session != nil {
		pair, err := m.RefreshTokenPair(ctx, RefreshTokenInput{RefreshToken: refreshToken})
		if err != nil {
			return "", err
		}
		return pair.AccessToken, nil
	}
	// Validate Refresh Token
	claims, err := m.VerifyToken(ctx, refreshToken)
	if err != nil {
		if m.metrics != nil {
			m.metrics.RecordRefreshed(ctx, "error")
		}
		return "", fmt.Errorf("invalid refresh token: %w", err)
	}

	// Check token type
	if claims.TokenType != "refresh" {
		if m.metrics != nil {
			m.metrics.RecordRefreshed(ctx, "invalid_type")
		}
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

	// Record Metrics
	if m.metrics != nil {
		m.metrics.RecordRefreshed(ctx, "success")
	}

	return accessToken, nil
}

// RevokeToken revoke token
func (m *tokenManagerImpl) RevokeToken(ctx context.Context, tokenString string) error {
	claims, err := m.parseSignedToken(ctx, tokenString)
	if err != nil {
		return nil
	}
	if m.session == nil || claims.SID == "" {
		return ErrSessionRequired
	}
	err = m.session.RevokeSession(ctx, claims.SID, "token_revoked")
	if err != nil {
		return err
	}

	// Record Metrics
	if m.metrics != nil {
		m.metrics.RecordRevoked(ctx)
	}

	return nil
}

// RevokeUserTokens Revoke all user tokens
func (m *tokenManagerImpl) RevokeUserTokens(ctx context.Context, subject string) error {
	if m.session == nil {
		return ErrSessionRequired
	}
	err := m.session.RevokeSubjectSessions(ctx, subject, "subject_revoked")
	if err != nil {
		return err
	}
	m.logger.InfoCtx(ctx, "user tokens revoked",
		zap.String("subject", subject),
	)

	return nil
}

func (m *tokenManagerImpl) RevokeSession(ctx context.Context, sid string, reason string) error {
	if m.session == nil {
		return ErrSessionRequired
	}
	return m.session.RevokeSession(ctx, sid, reason)
}

func (m *tokenManagerImpl) RevokeSubjectSessions(ctx context.Context, subject string, reason string) error {
	if m.session == nil {
		return ErrSessionRequired
	}
	return m.session.RevokeSubjectSessions(ctx, subject, reason)
}

func (m *tokenManagerImpl) ListSubjectSessions(ctx context.Context, subject string) ([]Session, error) {
	if m.session == nil {
		return nil, ErrSessionRequired
	}
	return m.session.ListSubjectSessions(ctx, subject)
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
	if sid, ok := mapClaims["sid"].(string); ok {
		claims.SID = sid
	}
	if ati, ok := mapClaims["ati"].(string); ok {
		claims.ATI = ati
	}
	if rti, ok := mapClaims["rti"].(string); ok {
		claims.RTI = rti
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

	// Validate Claims with the configured clock skew, consistent with the
	// parser leeway applied in VerifyToken (Claims.Valid() is strict and
	// would reject tokens the parser already accepted within the skew).
	if err := m.validateClaimsWithSkew(claims); err != nil {
		return nil, err
	}

	return claims, nil
}

// validateClaimsWithSkew re-checks exp/nbf with the configured clock skew,
// matching the leeway used by jwt.Parse in VerifyToken.
func (m *tokenManagerImpl) validateClaimsWithSkew(c *Claims) error {
	skew := m.config.Security.ClockSkew
	now := time.Now()

	if !c.ExpiresAt.IsZero() && now.After(c.ExpiresAt.Add(skew)) {
		return ErrTokenExpired
	}
	if !c.NotBefore.IsZero() && now.Before(c.NotBefore.Add(-skew)) {
		return ErrTokenNotYetValid
	}
	return nil
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
