package auth

import (
	"context"
	"strings"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

// PasswordService password service
type PasswordService struct {
	policy     PasswordPolicy
	bcryptCost int
	metrics    *AuthMetrics // Optional: metrics provider (injected after creation)
}

// Create password service
func NewPasswordService(policy PasswordPolicy, bcryptCost int) *PasswordService {
	return &PasswordService{
		policy:     policy,
		bcryptCost: bcryptCost,
	}
}

// SetMetrics injects the Auth metrics provider.
// This should be called after the PasswordService is created when metrics are enabled.
func (s *PasswordService) SetMetrics(metrics *AuthMetrics) {
	s.metrics = metrics
}

// HashPassword hash password (bcrypt)
func (s *PasswordService) HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.bcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword validate password
func (s *PasswordService) CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// ValidatePassword verify password policy
func (s *PasswordService) ValidatePassword(password string) error {
	return s.ValidatePasswordCtx(context.Background(), password)
}

// ValidatePasswordCtx verify password policy with context (for metrics)
func (s *PasswordService) ValidatePasswordCtx(ctx context.Context, password string) error {
	// Length check
	if len(password) < s.policy.MinLength {
		s.recordPasswordValidation(ctx, "too_short")
		return ErrPasswordTooShort
	}
	if len(password) > s.policy.MaxLength {
		s.recordPasswordValidation(ctx, "too_long")
		return ErrPasswordTooLong
	}

	// 2. Complexity Check
	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, ch := range password {
		switch {
		case unicode.IsUpper(ch):
			hasUpper = true
		case unicode.IsLower(ch):
			hasLower = true
		case unicode.IsDigit(ch):
			hasDigit = true
		case unicode.IsPunct(ch) || unicode.IsSymbol(ch):
			hasSpecial = true
		}
	}

	if s.policy.RequireUppercase && !hasUpper {
		s.recordPasswordValidation(ctx, "missing_uppercase")
		return ErrPasswordRequireUppercase
	}
	if s.policy.RequireLowercase && !hasLower {
		s.recordPasswordValidation(ctx, "missing_lowercase")
		return ErrPasswordRequireLowercase
	}
	if s.policy.RequireDigit && !hasDigit {
		s.recordPasswordValidation(ctx, "missing_digit")
		return ErrPasswordRequireDigit
	}
	if s.policy.RequireSpecialChar && !hasSpecial {
		s.recordPasswordValidation(ctx, "missing_special")
		return ErrPasswordRequireSpecial
	}

	// 3. Blacklist check
	passwordLower := strings.ToLower(password)
	for _, weak := range s.policy.Blacklist {
		if strings.Contains(passwordLower, strings.ToLower(weak)) {
			s.recordPasswordValidation(ctx, "blacklisted")
			return ErrPasswordInBlacklist
		}
	}

	s.recordPasswordValidation(ctx, "valid")
	return nil
}

// recordPasswordValidation helper to record password validation metrics
func (s *PasswordService) recordPasswordValidation(ctx context.Context, result string) {
	if s.metrics != nil {
		s.metrics.RecordPasswordValidation(ctx, result)
	}
}

// GetPolicy Retrieve password policy
func (s *PasswordService) GetPolicy() PasswordPolicy {
	return s.policy
}
