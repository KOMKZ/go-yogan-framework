package auth

import (
	"strings"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

// PasswordService 密码服务
type PasswordService struct {
	policy     PasswordPolicy
	bcryptCost int
}

// NewPasswordService 创建密码服务
func NewPasswordService(policy PasswordPolicy, bcryptCost int) *PasswordService {
	return &PasswordService{
		policy:     policy,
		bcryptCost: bcryptCost,
	}
}

// HashPassword 加密密码（bcrypt）
func (s *PasswordService) HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.bcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword 验证密码
func (s *PasswordService) CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// ValidatePassword 验证密码策略
func (s *PasswordService) ValidatePassword(password string) error {
	// 1. 长度检查
	if len(password) < s.policy.MinLength {
		return ErrPasswordTooShort
	}
	if len(password) > s.policy.MaxLength {
		return ErrPasswordTooLong
	}

	// 2. 复杂度检查
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

	// 3. 黑名单检查
	passwordLower := strings.ToLower(password)
	for _, weak := range s.policy.Blacklist {
		if strings.Contains(passwordLower, strings.ToLower(weak)) {
			return ErrPasswordInBlacklist
		}
	}

	return nil
}

// GetPolicy 获取密码策略
func (s *PasswordService) GetPolicy() PasswordPolicy {
	return s.policy
}
