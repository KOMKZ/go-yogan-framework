package jwt

import "time"

// Claims JWT 声明
type Claims struct {
	// 标准 Claims (JWT RFC 7519)
	Subject   string    `json:"sub"`           // 用户唯一标识
	IssuedAt  time.Time `json:"iat"`           // 签发时间
	ExpiresAt time.Time `json:"exp"`           // 过期时间
	NotBefore time.Time `json:"nbf,omitempty"` // 生效时间
	Issuer    string    `json:"iss,omitempty"` // 签发者
	Audience  string    `json:"aud,omitempty"` // 接收方
	JTI       string    `json:"jti,omitempty"` // Token ID（防重放）

	// 自定义 Claims（应用层扩展）
	UserID    int64                  `json:"user_id,omitempty"`
	Username  string                 `json:"username,omitempty"`
	Roles     []string               `json:"roles,omitempty"`
	TenantID  string                 `json:"tenant_id,omitempty"`
	TokenType string                 `json:"token_type,omitempty"` // access, refresh
	Extra     map[string]interface{} `json:"extra,omitempty"`
}

// Valid 验证 Claims 是否有效（实现 jwt.Claims 接口）
func (c *Claims) Valid() error {
	now := time.Now()

	// 检查过期时间
	if !c.ExpiresAt.IsZero() && now.After(c.ExpiresAt) {
		return ErrTokenExpired
	}

	// 检查生效时间
	if !c.NotBefore.IsZero() && now.Before(c.NotBefore) {
		return ErrTokenNotYetValid
	}

	return nil
}

// IsExpired 判断 token 是否过期
func (c *Claims) IsExpired() bool {
	if c.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(c.ExpiresAt)
}

// TTL 返回剩余有效时间
func (c *Claims) TTL() time.Duration {
	if c.ExpiresAt.IsZero() {
		return 0
	}
	return time.Until(c.ExpiresAt)
}
