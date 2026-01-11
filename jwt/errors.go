package jwt

import "errors"

var (
	// ErrTokenMissing Token 缺失
	ErrTokenMissing = errors.New("jwt: token missing")

	// ErrTokenInvalid token 无效
	ErrTokenInvalid = errors.New("jwt: token invalid")

	// ErrTokenExpired token 已过期
	ErrTokenExpired = errors.New("jwt: token expired")

	// ErrTokenNotYetValid token 尚未生效
	ErrTokenNotYetValid = errors.New("jwt: token not yet valid")

	// ErrInvalidSignature 签名无效
	ErrInvalidSignature = errors.New("jwt: invalid signature")

	// ErrTokenBlacklisted token 已被撤销
	ErrTokenBlacklisted = errors.New("jwt: token blacklisted")

	// ErrInvalidClaims claims 无效
	ErrInvalidClaims = errors.New("jwt: invalid claims")

	// ErrSecretEmpty 密钥为空
	ErrSecretEmpty = errors.New("jwt: secret is empty")

	// ErrAlgorithmNotSupported 不支持的算法
	ErrAlgorithmNotSupported = errors.New("jwt: algorithm not supported")
)
