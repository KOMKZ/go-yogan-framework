package jwt

import "errors"

var (
	// ErrTokenMissing Token missing
	ErrTokenMissing = errors.New("jwt: token missing")

	// ErrTokenInvalid token is invalid
	ErrTokenInvalid = errors.New("jwt: token invalid")

	// ErrTokenExpired token has expired
	ErrTokenExpired = errors.New("jwt: token expired")

	// ErrTokenNotYetValid token has not yet taken effect
	ErrTokenNotYetValid = errors.New("jwt: token not yet valid")

	// ErrInvalidSignature Invalid signature
	ErrInvalidSignature = errors.New("jwt: invalid signature")

	// ErrTokenBlacklisted token has been revoked
	ErrTokenBlacklisted = errors.New("jwt: token blacklisted")

	// ErrInvalidClaims claims invalid
	ErrInvalidClaims = errors.New("jwt: invalid claims")

	// ErrSecretEmpty Secret is empty
	ErrSecretEmpty = errors.New("jwt: secret is empty")

	// ErrAlgorithmNotSupported Algorithm not supported
	ErrAlgorithmNotSupported = errors.New("jwt: algorithm not supported")
)
