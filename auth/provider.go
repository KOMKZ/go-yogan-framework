package auth

import (
	"context"
	"time"
)

// AuthProvider 认证提供者接口
type AuthProvider interface {
	// Name 认证方式名称（password, oauth2, api_key, basic_auth）
	Name() string
	
	// Authenticate 执行认证
	Authenticate(ctx context.Context, credentials Credentials) (*AuthResult, error)
}

// Credentials 认证凭证（通用结构）
type Credentials struct {
	// 用户名/密码认证
	Username string
	Password string
	
	// OAuth2.0 认证
	AuthCode string
	Provider string // google, github, wechat
	
	// API Key 认证
	APIKey string
	
	// Basic Auth 认证
	BasicAuthUser string
	BasicAuthPass string
}

// AuthResult 认证结果
type AuthResult struct {
	UserID   int64                  // 用户 ID
	Username string                 // 用户名
	Email    string                 // 邮箱
	Roles    []string               // 角色列表
	Extra    map[string]interface{} // 额外信息
}

// PasswordAuthProvider 密码认证提供者
type PasswordAuthProvider struct {
	passwordService *PasswordService
	userRepository  UserRepository
	attemptStore    LoginAttemptStore
	maxAttempts     int
	lockoutDuration time.Duration
}

// UserRepository 用户仓库接口（业务层实现）
type UserRepository interface {
	// FindByUsername 根据用户名查找用户
	FindByUsername(ctx context.Context, username string) (*User, error)
	
	// FindByEmail 根据邮箱查找用户
	FindByEmail(ctx context.Context, email string) (*User, error)
}

// User 用户模型
type User struct {
	ID           int64
	Username     string
	Email        string
	PasswordHash string
	Status       string // active, inactive, banned
	Roles        []string
}

// NewPasswordAuthProvider 创建密码认证提供者
func NewPasswordAuthProvider(
	passwordService *PasswordService,
	userRepository UserRepository,
	attemptStore LoginAttemptStore,
	maxAttempts int,
	lockoutDuration time.Duration,
) *PasswordAuthProvider {
	return &PasswordAuthProvider{
		passwordService: passwordService,
		userRepository:  userRepository,
		attemptStore:    attemptStore,
		maxAttempts:     maxAttempts,
		lockoutDuration: lockoutDuration,
	}
}

// Name 认证方式名称
func (p *PasswordAuthProvider) Name() string {
	return "password"
}

// Authenticate 执行密码认证
func (p *PasswordAuthProvider) Authenticate(ctx context.Context, credentials Credentials) (*AuthResult, error) {
	username := credentials.Username
	password := credentials.Password

	// 1. 检查登录尝试次数
	if p.attemptStore != nil {
		locked, err := p.attemptStore.IsLocked(ctx, username, p.maxAttempts)
		if err == nil && locked {
			return nil, ErrTooManyAttempts
		}
	}

	// 2. 查询用户
	user, err := p.userRepository.FindByUsername(ctx, username)
	if err != nil {
		// 增加失败次数（防止用户名枚举）
		if p.attemptStore != nil {
			p.attemptStore.IncrementAttempts(ctx, username, p.lockoutDuration)
		}
		return nil, ErrInvalidCredentials
	}

	// 3. 验证密码
	if !p.passwordService.CheckPassword(password, user.PasswordHash) {
		if p.attemptStore != nil {
			p.attemptStore.IncrementAttempts(ctx, username, p.lockoutDuration)
		}
		return nil, ErrInvalidCredentials
	}

	// 4. 检查账户状态
	if user.Status != "active" {
		return nil, ErrAccountDisabled
	}

	// 5. 重置登录尝试
	if p.attemptStore != nil {
		p.attemptStore.ResetAttempts(ctx, username)
	}

	// 6. 返回认证结果
	return &AuthResult{
		UserID:   user.ID,
		Username: user.Username,
		Email:    user.Email,
		Roles:    user.Roles,
	}, nil
}

