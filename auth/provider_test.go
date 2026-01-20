package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockUserRepository simulated user repository
type mockUserRepository struct {
	users       map[string]*User
	findByUsername func(ctx context.Context, username string) (*User, error)
}

func newMockUserRepository() *mockUserRepository {
	return &mockUserRepository{
		users: make(map[string]*User),
	}
}

func (r *mockUserRepository) FindByUsername(ctx context.Context, username string) (*User, error) {
	if r.findByUsername != nil {
		return r.findByUsername(ctx, username)
	}
	user, ok := r.users[username]
	if !ok {
		return nil, errors.New("user not found")
	}
	return user, nil
}

func (r *mockUserRepository) FindByEmail(ctx context.Context, email string) (*User, error) {
	for _, user := range r.users {
		if user.Email == email {
			return user, nil
		}
	}
	return nil, errors.New("user not found")
}

func (r *mockUserRepository) AddUser(user *User) {
	r.users[user.Username] = user
}

// mockLoginAttemptStore simulated login attempt storage
type mockLoginAttemptStore struct {
	attempts   map[string]int
	isLocked   bool
	lockErr    error
	incrementErr error
}

func newMockLoginAttemptStore() *mockLoginAttemptStore {
	return &mockLoginAttemptStore{
		attempts: make(map[string]int),
	}
}

func (s *mockLoginAttemptStore) GetAttempts(ctx context.Context, username string) (int, error) {
	return s.attempts[username], nil
}

func (s *mockLoginAttemptStore) IncrementAttempts(ctx context.Context, username string, ttl time.Duration) error {
	if s.incrementErr != nil {
		return s.incrementErr
	}
	s.attempts[username]++
	return nil
}

func (s *mockLoginAttemptStore) ResetAttempts(ctx context.Context, username string) error {
	delete(s.attempts, username)
	return nil
}

func (s *mockLoginAttemptStore) IsLocked(ctx context.Context, username string, maxAttempts int) (bool, error) {
	if s.lockErr != nil {
		return false, s.lockErr
	}
	if s.isLocked {
		return true, nil
	}
	return s.attempts[username] >= maxAttempts, nil
}

func (s *mockLoginAttemptStore) Close() error {
	return nil
}

func TestNewPasswordAuthProvider(t *testing.T) {
	policy := PasswordPolicy{MinLength: 8, MaxLength: 128}
	pwdService := NewPasswordService(policy, 10)
	userRepo := newMockUserRepository()
	attemptStore := newMockLoginAttemptStore()

	provider := NewPasswordAuthProvider(
		pwdService,
		userRepo,
		attemptStore,
		5,
		30*time.Minute,
	)

	assert.NotNil(t, provider)
	assert.Equal(t, pwdService, provider.passwordService)
	assert.Equal(t, userRepo, provider.userRepository)
	assert.Equal(t, attemptStore, provider.attemptStore)
	assert.Equal(t, 5, provider.maxAttempts)
	assert.Equal(t, 30*time.Minute, provider.lockoutDuration)
}

func TestPasswordAuthProvider_Name(t *testing.T) {
	provider := &PasswordAuthProvider{}
	assert.Equal(t, "password", provider.Name())
}

func TestPasswordAuthProvider_Authenticate(t *testing.T) {
	ctx := context.Background()

	createProvider := func(userRepo *mockUserRepository, attemptStore *mockLoginAttemptStore) *PasswordAuthProvider {
		policy := PasswordPolicy{MinLength: 8, MaxLength: 128}
		pwdService := NewPasswordService(policy, 4) // use low cost for fast tests
		return NewPasswordAuthProvider(
			pwdService,
			userRepo,
			attemptStore,
			5,
			30*time.Minute,
		)
	}

	t.Run("success", func(t *testing.T) {
		userRepo := newMockUserRepository()
		attemptStore := newMockLoginAttemptStore()
		provider := createProvider(userRepo, attemptStore)

		// Create password hash
		hash, _ := provider.passwordService.HashPassword("ValidPass123")
		userRepo.AddUser(&User{
			ID:           1,
			Username:     "testuser",
			Email:        "test@example.com",
			PasswordHash: hash,
			Status:       "active",
			Roles:        []string{"user"},
		})

		result, err := provider.Authenticate(ctx, Credentials{
			Username: "testuser",
			Password: "ValidPass123",
		})

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, int64(1), result.UserID)
		assert.Equal(t, "testuser", result.Username)
		assert.Equal(t, "test@example.com", result.Email)
		assert.Equal(t, []string{"user"}, result.Roles)
	})

	t.Run("account locked", func(t *testing.T) {
		userRepo := newMockUserRepository()
		attemptStore := newMockLoginAttemptStore()
		attemptStore.isLocked = true
		provider := createProvider(userRepo, attemptStore)

		result, err := provider.Authenticate(ctx, Credentials{
			Username: "lockeduser",
			Password: "password",
		})

		assert.Nil(t, result)
		assert.ErrorIs(t, err, ErrTooManyAttempts)
	})

	t.Run("user not found", func(t *testing.T) {
		userRepo := newMockUserRepository()
		attemptStore := newMockLoginAttemptStore()
		provider := createProvider(userRepo, attemptStore)

		result, err := provider.Authenticate(ctx, Credentials{
			Username: "nonexistent",
			Password: "password",
		})

		assert.Nil(t, result)
		assert.ErrorIs(t, err, ErrInvalidCredentials)
		// Should increment attempts
		assert.Equal(t, 1, attemptStore.attempts["nonexistent"])
	})

	t.Run("wrong password", func(t *testing.T) {
		userRepo := newMockUserRepository()
		attemptStore := newMockLoginAttemptStore()
		provider := createProvider(userRepo, attemptStore)

		hash, _ := provider.passwordService.HashPassword("CorrectPass123")
		userRepo.AddUser(&User{
			ID:           1,
			Username:     "testuser",
			Email:        "test@example.com",
			PasswordHash: hash,
			Status:       "active",
		})

		result, err := provider.Authenticate(ctx, Credentials{
			Username: "testuser",
			Password: "WrongPassword",
		})

		assert.Nil(t, result)
		assert.ErrorIs(t, err, ErrInvalidCredentials)
		// Should increment attempts
		assert.Equal(t, 1, attemptStore.attempts["testuser"])
	})

	t.Run("account disabled", func(t *testing.T) {
		userRepo := newMockUserRepository()
		attemptStore := newMockLoginAttemptStore()
		provider := createProvider(userRepo, attemptStore)

		hash, _ := provider.passwordService.HashPassword("ValidPass123")
		userRepo.AddUser(&User{
			ID:           1,
			Username:     "disableduser",
			Email:        "disabled@example.com",
			PasswordHash: hash,
			Status:       "inactive",
		})

		result, err := provider.Authenticate(ctx, Credentials{
			Username: "disableduser",
			Password: "ValidPass123",
		})

		assert.Nil(t, result)
		assert.ErrorIs(t, err, ErrAccountDisabled)
	})

	t.Run("without attempt store", func(t *testing.T) {
		userRepo := newMockUserRepository()
		policy := PasswordPolicy{MinLength: 8, MaxLength: 128}
		pwdService := NewPasswordService(policy, 4)
		provider := NewPasswordAuthProvider(
			pwdService,
			userRepo,
			nil, // no attempt store
			5,
			30*time.Minute,
		)

		hash, _ := provider.passwordService.HashPassword("ValidPass123")
		userRepo.AddUser(&User{
			ID:           1,
			Username:     "testuser",
			Email:        "test@example.com",
			PasswordHash: hash,
			Status:       "active",
		})

		result, err := provider.Authenticate(ctx, Credentials{
			Username: "testuser",
			Password: "ValidPass123",
		})

		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("user not found without attempt store", func(t *testing.T) {
		userRepo := newMockUserRepository()
		policy := PasswordPolicy{MinLength: 8, MaxLength: 128}
		pwdService := NewPasswordService(policy, 4)
		provider := NewPasswordAuthProvider(
			pwdService,
			userRepo,
			nil, // no attempt store
			5,
			30*time.Minute,
		)

		result, err := provider.Authenticate(ctx, Credentials{
			Username: "nonexistent",
			Password: "password",
		})

		assert.Nil(t, result)
		assert.ErrorIs(t, err, ErrInvalidCredentials)
	})

	t.Run("wrong password without attempt store", func(t *testing.T) {
		userRepo := newMockUserRepository()
		policy := PasswordPolicy{MinLength: 8, MaxLength: 128}
		pwdService := NewPasswordService(policy, 4)
		provider := NewPasswordAuthProvider(
			pwdService,
			userRepo,
			nil, // no attempt store
			5,
			30*time.Minute,
		)

		hash, _ := provider.passwordService.HashPassword("CorrectPass123")
		userRepo.AddUser(&User{
			ID:           1,
			Username:     "testuser",
			PasswordHash: hash,
			Status:       "active",
		})

		result, err := provider.Authenticate(ctx, Credentials{
			Username: "testuser",
			Password: "WrongPassword",
		})

		assert.Nil(t, result)
		assert.ErrorIs(t, err, ErrInvalidCredentials)
	})
}

func TestCredentials(t *testing.T) {
	creds := Credentials{
		Username:      "user",
		Password:      "pass",
		AuthCode:      "code",
		Provider:      "google",
		APIKey:        "key123",
		BasicAuthUser: "basic_user",
		BasicAuthPass: "basic_pass",
	}

	assert.Equal(t, "user", creds.Username)
	assert.Equal(t, "pass", creds.Password)
	assert.Equal(t, "code", creds.AuthCode)
	assert.Equal(t, "google", creds.Provider)
	assert.Equal(t, "key123", creds.APIKey)
	assert.Equal(t, "basic_user", creds.BasicAuthUser)
	assert.Equal(t, "basic_pass", creds.BasicAuthPass)
}

func TestAuthResult(t *testing.T) {
	result := &AuthResult{
		UserID:   100,
		Username: "testuser",
		Email:    "test@example.com",
		Roles:    []string{"admin", "user"},
		Extra:    map[string]interface{}{"key": "value"},
	}

	assert.Equal(t, int64(100), result.UserID)
	assert.Equal(t, "testuser", result.Username)
	assert.Equal(t, "test@example.com", result.Email)
	assert.Equal(t, []string{"admin", "user"}, result.Roles)
	assert.Equal(t, "value", result.Extra["key"])
}

func TestUser(t *testing.T) {
	user := &User{
		ID:           1,
		Username:     "testuser",
		Email:        "test@example.com",
		PasswordHash: "hash",
		Status:       "active",
		Roles:        []string{"admin"},
	}

	assert.Equal(t, int64(1), user.ID)
	assert.Equal(t, "testuser", user.Username)
	assert.Equal(t, "test@example.com", user.Email)
	assert.Equal(t, "hash", user.PasswordHash)
	assert.Equal(t, "active", user.Status)
	assert.Equal(t, []string{"admin"}, user.Roles)
}
