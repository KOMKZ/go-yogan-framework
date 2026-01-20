package auth

import (
	"strings"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

// PasswordService password service
type PasswordService struct {
	policy     PasswordPolicy
	bcryptCost int
}

// Create password service
func NewPasswordService(policy PasswordPolicy, bcryptCost int) *PasswordService {
	return &PasswordService{
		policy:     policy,
		bcryptCost: bcryptCost,
	}
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
	// Length check
	if len(password) < s.policy.MinLength {
		return ErrPasswordTooShort
	}
	if len(password) > s.policy.MaxLength {
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
		return ErrPasswordRequireUppercase
	}
	if s.policy.RequireLowercase && !hasLower {
		return ErrPasswordRequireLowercase
	}
	if s.policy.RequireDigit && !hasDigit {
		return ErrPasswordRequireDigit
	}
	if s.policy.RequireSpecialChar && !hasSpecial {
		return ErrPasswordRequireSpecial
	}

	// 3. Blacklist check
	passwordLower := strings.ToLower(password)
	for _, weak := range s.policy.Blacklist {
		if strings.Contains(passwordLower, strings.ToLower(weak)) {
			return ErrPasswordInBlacklist
		}
	}

	return nil
}

// GetPolicy Retrieve password policy
func (s *PasswordService) GetPolicy() PasswordPolicy {
	return s.policy
}
