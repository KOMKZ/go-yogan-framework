package auth

import "errors"

// Authentication related errors
var (
	// Password error
	ErrPasswordTooShort         = errors.New("Password length is too short")
	ErrPasswordTooLong          = errors.New("Password length is too long")
	ErrPasswordRequireUppercase = errors.New("Password must contain uppercase letters")
	ErrPasswordRequireLowercase = errors.New("The password must contain lowercase letters")
	ErrPasswordRequireDigit     = errors.New("The password must contain numbers")
	ErrPasswordRequireSpecial   = errors.New("The password must contain special characters")
	ErrPasswordTooWeak          = errors.New("Password is too simple, please use a stronger password，Password is too simple, please use a stronger password")
	ErrPasswordInBlacklist      = errors.New("Password is in the blacklist")
	
	// Login error
	ErrInvalidCredentials       = errors.New("Username or password incorrect")
	ErrUserNotFound             = errors.New("User does not exist")
	ErrAccountDisabled          = errors.New("Account disabled")
	ErrAccountLocked            = errors.New("Account is locked")
	ErrTooManyAttempts          = errors.New("Too many login attempts, please try again later，Too many login attempts, please try again later")
	
	// Authentication provider error
	ErrProviderNotEnabled       = errors.New("Authentication method not enabled")
	ErrProviderNotSupported     = errors.New("Unsupported authentication method")
)

