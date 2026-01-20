package auth

import "errors"

// Authentication related errors
var (
	// Password error
	ErrPasswordTooShort         = errors.New("密码长度过短")
	ErrPasswordTooLong          = errors.New("密码长度过长")
	ErrPasswordRequireUppercase = errors.New("密码必须包含大写字母")
	ErrPasswordRequireLowercase = errors.New("密码必须包含小写字母")
	ErrPasswordRequireDigit     = errors.New("密码必须包含数字")
	ErrPasswordRequireSpecial   = errors.New("密码必须包含特殊字符")
	ErrPasswordTooWeak          = errors.New("密码过于简单，请使用更强密码")
	ErrPasswordInBlacklist      = errors.New("密码在黑名单中")
	
	// Login error
	ErrInvalidCredentials       = errors.New("用户名或密码错误")
	ErrUserNotFound             = errors.New("用户不存在")
	ErrAccountDisabled          = errors.New("账户已禁用")
	ErrAccountLocked            = errors.New("账户已锁定")
	ErrTooManyAttempts          = errors.New("登录尝试次数过多，请稍后再试")
	
	// Authentication provider error
	ErrProviderNotEnabled       = errors.New("认证方式未启用")
	ErrProviderNotSupported     = errors.New("不支持的认证方式")
)

