package jwt

import "time"

// Claims JWT statements
type Claims struct {
	// Standard Claims (JWT RFC 7519)
	Subject   string    `json:"sub"`           // User unique identifier
	IssuedAt  time.Time `json:"iat"`           // issue time
	ExpiresAt time.Time `json:"exp"`           // expiration time
	NotBefore time.Time `json:"nbf,omitempty"` // Effective date
	Issuer    string    `json:"iss,omitempty"` // issuer
	Audience  string    `json:"aud,omitempty"` // receiver
	JTI       string    `json:"jti,omitempty"` // Token ID (anti-replay)

	// Custom Claims (application layer extensions)
	UserID    int64                  `json:"user_id,omitempty"`
	Username  string                 `json:"username,omitempty"`
	Roles     []string               `json:"roles,omitempty"`
	TenantID  string                 `json:"tenant_id,omitempty"`
	TokenType string                 `json:"token_type,omitempty"` // access, refresh
	Extra     map[string]interface{} `json:"extra,omitempty"`
}

// Validate whether Claims are valid (implement jwt.Claims interface)
func (c *Claims) Valid() error {
	now := time.Now()

	// Check expiration time
	if !c.ExpiresAt.IsZero() && now.After(c.ExpiresAt) {
		return ErrTokenExpired
	}

	// Check effective time
	if !c.NotBefore.IsZero() && now.Before(c.NotBefore) {
		return ErrTokenNotYetValid
	}

	return nil
}

// Check if token has expired
func (c *Claims) IsExpired() bool {
	if c.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(c.ExpiresAt)
}

// TTL returns remaining valid time
func (c *Claims) TTL() time.Duration {
	if c.ExpiresAt.IsZero() {
		return 0
	}
	return time.Until(c.ExpiresAt)
}
