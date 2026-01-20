package auth

import (
	"context"
	"time"
)

// Authentication provider interface
type AuthProvider interface {
	// Name Authentication method name (password, oauth2, api_key, basic_auth)
	Name() string
	
	// Authenticate execution authorization
	Authenticate(ctx context.Context, credentials Credentials) (*AuthResult, error)
}

// Credentials (generic structure)
type Credentials struct {
	// username/password authentication
	Username string
	Password string
	
	// OAuth 2.0 authentication
	AuthCode string
	Provider string // google, github, wechat
	
	// API key authentication
	APIKey string
	
	// Basic Authentication认证
	BasicAuthUser string
	BasicAuthPass string
}

// Authentication result
type AuthResult struct {
	UserID   int64                  // User ID
	Username string                 // Username
	Email    string                 // emailADDRESS
	Roles    []string               // List of characters
	Extra    map[string]interface{} // Additional information
}

// PasswordAuthProvider password authentication provider
type PasswordAuthProvider struct {
	passwordService *PasswordService
	userRepository  UserRepository
	attemptStore    LoginAttemptStore
	maxAttempts     int
	lockoutDuration time.Duration
}

// UserRepository user repository interface (business layer implementation)
type UserRepository interface {
	// FindUserByUserName retrieves user by username
	FindByUsername(ctx context.Context, username string) (*User, error)
	
	// Find user by email
	FindByEmail(ctx context.Context, email string) (*User, error)
}

// User model
type User struct {
	ID           int64
	Username     string
	Email        string
	PasswordHash string
	Status       string // active, inactive, banned
	Roles        []string
}

// Create password authentication provider
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

// Name Authentication method name
func (p *PasswordAuthProvider) Name() string {
	return "password"
}

// Authenticate password authentication execution
func (p *PasswordAuthProvider) Authenticate(ctx context.Context, credentials Credentials) (*AuthResult, error) {
	username := credentials.Username
	password := credentials.Password

	// 1. Check login attempt count
	if p.attemptStore != nil {
		locked, err := p.attemptStore.IsLocked(ctx, username, p.maxAttempts)
		if err == nil && locked {
			return nil, ErrTooManyAttempts
		}
	}

	// 2. Query user
	user, err := p.userRepository.FindByUsername(ctx, username)
	if err != nil {
		// Increase failure count (prevent username enumeration)
		if p.attemptStore != nil {
			p.attemptStore.IncrementAttempts(ctx, username, p.lockoutDuration)
		}
		return nil, ErrInvalidCredentials
	}

	// 3. Verify password
	if !p.passwordService.CheckPassword(password, user.PasswordHash) {
		if p.attemptStore != nil {
			p.attemptStore.IncrementAttempts(ctx, username, p.lockoutDuration)
		}
		return nil, ErrInvalidCredentials
	}

	// 4. Check account status
	if user.Status != "active" {
		return nil, ErrAccountDisabled
	}

	// 5. Reset login attempts
	if p.attemptStore != nil {
		p.attemptStore.ResetAttempts(ctx, username)
	}

	// 6. Return authentication result
	return &AuthResult{
		UserID:   user.ID,
		Username: user.Username,
		Email:    user.Email,
		Roles:    user.Roles,
	}, nil
}

