package auth

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestPasswordService_HashPassword(t *testing.T) {
	policy := PasswordPolicy{
		MinLength: 8,
		MaxLength: 128,
	}
	service := NewPasswordService(policy, 12)

	password := "TestPassword123"
	hash, err := service.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	// Validate that the hash is not empty
	if hash == "" {
		t.Error("HashPassword() returned empty hash")
	}

	// verify that the hash can be verified
	if !service.CheckPassword(password, hash) {
		t.Error("CheckPassword() failed for valid password")
	}

	// Verify that incorrect passwords do not pass
	if service.CheckPassword("WrongPassword", hash) {
		t.Error("CheckPassword() passed for invalid password")
	}
}

func TestPasswordService_CheckPassword(t *testing.T) {
	policy := PasswordPolicy{}
	service := NewPasswordService(policy, 12)

	password := "MySecretPassword"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), 12)

	tests := []struct {
		name     string
		password string
		hash     string
		want     bool
	}{
		{
			name:     "valid password",
			password: password,
			hash:     string(hash),
			want:     true,
		},
		{
			name:     "invalid password",
			password: "WrongPassword",
			hash:     string(hash),
			want:     false,
		},
		{
			name:     "empty password",
			password: "",
			hash:     string(hash),
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := service.CheckPassword(tt.password, tt.hash)
			if got != tt.want {
				t.Errorf("CheckPassword() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPasswordService_ValidatePassword(t *testing.T) {
	policy := PasswordPolicy{
		MinLength:          8,
		MaxLength:          128,
		RequireUppercase:   true,
		RequireLowercase:   true,
		RequireDigit:       true,
		RequireSpecialChar: false,
		Blacklist:          []string{"password", "123456"},
	}

	service := NewPasswordService(policy, 12)

	tests := []struct {
		name     string
		password string
		wantErr  error
	}{
		{
			name:     "valid password",
			password: "Abc123def",
			wantErr:  nil,
		},
		{
			name:     "too short",
			password: "Abc1",
			wantErr:  ErrPasswordTooShort,
		},
		{
			name:     "too long",
			password: "Abc123" + string(make([]byte, 130)),
			wantErr:  ErrPasswordTooLong,
		},
		{
			name:     "no uppercase",
			password: "abc123def",
			wantErr:  ErrPasswordRequireUppercase,
		},
		{
			name:     "no lowercase",
			password: "ABC123DEF",
			wantErr:  ErrPasswordRequireLowercase,
		},
		{
			name:     "no digit",
			password: "AbcDefGhi",
			wantErr:  ErrPasswordRequireDigit,
		},
		{
			name:     "in blacklist - password",
			password: "Password123",
			wantErr:  ErrPasswordInBlacklist,
		},
		{
			name:     "in blacklist - 123456",
			password: "Abc123456",
			wantErr:  ErrPasswordInBlacklist,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.ValidatePassword(tt.password)
			if err != tt.wantErr {
				t.Errorf("ValidatePassword() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPasswordService_ValidatePassword_WithSpecialChar(t *testing.T) {
	policy := PasswordPolicy{
		MinLength:          8,
		MaxLength:          128,
		RequireUppercase:   true,
		RequireLowercase:   true,
		RequireDigit:       true,
		RequireSpecialChar: true,
	}

	service := NewPasswordService(policy, 12)

	tests := []struct {
		name     string
		password string
		wantErr  error
	}{
		{
			name:     "valid with special char",
			password: "Abc123!@#",
			wantErr:  nil,
		},
		{
			name:     "no special char",
			password: "Abc123def",
			wantErr:  ErrPasswordRequireSpecial,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.ValidatePassword(tt.password)
			if err != tt.wantErr {
				t.Errorf("ValidatePassword() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPasswordService_GetPolicy(t *testing.T) {
	policy := PasswordPolicy{
		MinLength: 10,
		MaxLength: 100,
	}

	service := NewPasswordService(policy, 12)
	gotPolicy := service.GetPolicy()

	if gotPolicy.MinLength != policy.MinLength {
		t.Errorf("GetPolicy().MinLength = %v, want %v", gotPolicy.MinLength, policy.MinLength)
	}
	if gotPolicy.MaxLength != policy.MaxLength {
		t.Errorf("GetPolicy().MaxLength = %v, want %v", gotPolicy.MaxLength, policy.MaxLength)
	}
}

