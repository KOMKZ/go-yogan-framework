// 本文件提供 Cookie 会话下的 CSRF 双提交保护。
// 上游由应用配置签名密钥和跳过规则；下游提供 token 签发与非安全方法校验。
// 维护时不要引入业务路由判断，只保留可复用的 HTTP 中间件能力。
package middleware

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	defaultCSRFHeaderName = "X-CSRF-Token"
	defaultCSRFCookieName = "_csrf_token_hash"
	defaultCSRFTokenTTL   = 2 * time.Hour
)

var (
	// ErrCSRFTokenMissing indicates that either the header token or digest cookie is missing.
	ErrCSRFTokenMissing = errors.New("csrf: token missing")
	// ErrCSRFTokenInvalid indicates that the token signature or cookie digest is invalid.
	ErrCSRFTokenInvalid = errors.New("csrf: token invalid")
	// ErrCSRFTokenExpired indicates that the signed token is outside its TTL.
	ErrCSRFTokenExpired = errors.New("csrf: token expired")
)

// CSRFConfig configures the double-submit CSRF middleware.
type CSRFConfig struct {
	Secret           []byte
	HeaderName       string
	CookieName       string
	TokenTTL         time.Duration
	CookiePath       string
	CookieMaxAge     int
	CookieHTTPOnly   bool
	CookieSecure     bool
	CookieSecureFunc func(*gin.Context) bool
	CookieSameSite   http.SameSite
	SafeMethods      []string
	Skipper          func(*gin.Context) bool
	ErrorHandler     func(*gin.Context, error)
}

// CSRFTokenResponse is the JSON payload returned by a CSRF token issuing route.
type CSRFTokenResponse struct {
	Token string `json:"csrf_token"`
}

// DefaultCSRFConfig builds a secure default config for cookie-backed web sessions.
func DefaultCSRFConfig(secret []byte) CSRFConfig {
	return CSRFConfig{
		Secret:         secret,
		HeaderName:     defaultCSRFHeaderName,
		CookieName:     defaultCSRFCookieName,
		TokenTTL:       defaultCSRFTokenTTL,
		CookiePath:     "/",
		CookieHTTPOnly: true,
		CookieSameSite: http.SameSiteStrictMode,
		SafeMethods:    []string{http.MethodGet, http.MethodHead, http.MethodOptions},
		ErrorHandler:   defaultCSRFErrorHandler,
	}
}

// CSRF validates signed header tokens for non-safe requests.
func CSRF(config CSRFConfig) gin.HandlerFunc {
	config = normalizeCSRFConfig(config)
	return func(c *gin.Context) {
		if isCSRFSafeMethod(c.Request.Method, config.SafeMethods) {
			c.Next()
			return
		}
		if config.Skipper != nil && config.Skipper(c) {
			c.Next()
			return
		}

		token := strings.TrimSpace(c.GetHeader(config.HeaderName))
		cookieValue, cookieErr := c.Cookie(config.CookieName)
		if token == "" || cookieErr != nil || strings.TrimSpace(cookieValue) == "" {
			config.ErrorHandler(c, ErrCSRFTokenMissing)
			return
		}

		if err := verifyCSRFToken(config.Secret, token, time.Now()); err != nil {
			config.ErrorHandler(c, err)
			return
		}
		expectedDigest := csrfCookieDigest(config.Secret, token)
		if subtle.ConstantTimeCompare([]byte(expectedDigest), []byte(cookieValue)) != 1 {
			config.ErrorHandler(c, ErrCSRFTokenInvalid)
			return
		}

		c.Next()
	}
}

// IssueCSRFToken signs a new token and writes its HttpOnly digest cookie.
func IssueCSRFToken(c *gin.Context, config CSRFConfig) (*CSRFTokenResponse, error) {
	config = normalizeCSRFConfig(config)
	token, err := newCSRFToken(config.Secret, time.Now().Add(config.TokenTTL))
	if err != nil {
		return nil, err
	}
	maxAge := config.CookieMaxAge
	if maxAge == 0 {
		maxAge = int(config.TokenTTL.Seconds())
	}
	secure := config.CookieSecure
	if config.CookieSecureFunc != nil {
		secure = config.CookieSecureFunc(c)
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     config.CookieName,
		Value:    csrfCookieDigest(config.Secret, token),
		Path:     config.CookiePath,
		MaxAge:   maxAge,
		HttpOnly: config.CookieHTTPOnly,
		Secure:   secure,
		SameSite: config.CookieSameSite,
	})
	return &CSRFTokenResponse{Token: token}, nil
}

func normalizeCSRFConfig(config CSRFConfig) CSRFConfig {
	defaults := DefaultCSRFConfig(config.Secret)
	if config.HeaderName == "" {
		config.HeaderName = defaults.HeaderName
	}
	if config.CookieName == "" {
		config.CookieName = defaults.CookieName
	}
	if config.TokenTTL <= 0 {
		config.TokenTTL = defaults.TokenTTL
	}
	if config.CookiePath == "" {
		config.CookiePath = defaults.CookiePath
	}
	if config.CookieSameSite == 0 {
		config.CookieSameSite = defaults.CookieSameSite
	}
	if len(config.SafeMethods) == 0 {
		config.SafeMethods = defaults.SafeMethods
	}
	if config.ErrorHandler == nil {
		config.ErrorHandler = defaults.ErrorHandler
	}
	return config
}

func newCSRFToken(secret []byte, expiresAt time.Time) (string, error) {
	if len(secret) == 0 {
		return "", ErrCSRFTokenInvalid
	}
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	expiry := strconv.FormatInt(expiresAt.Unix(), 10)
	noncePart := base64.RawURLEncoding.EncodeToString(nonce)
	payload := expiry + "." + noncePart
	signature := csrfSign(secret, "token|"+payload)
	return payload + "." + signature, nil
}

func verifyCSRFToken(secret []byte, token string, now time.Time) error {
	if len(secret) == 0 {
		return ErrCSRFTokenInvalid
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return ErrCSRFTokenInvalid
	}
	expiry, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || expiry <= 0 {
		return ErrCSRFTokenInvalid
	}
	if now.Unix() > expiry {
		return ErrCSRFTokenExpired
	}
	payload := parts[0] + "." + parts[1]
	expected := csrfSign(secret, "token|"+payload)
	if subtle.ConstantTimeCompare([]byte(expected), []byte(parts[2])) != 1 {
		return ErrCSRFTokenInvalid
	}
	return nil
}

func csrfCookieDigest(secret []byte, token string) string {
	return csrfSign(secret, "cookie|"+token)
}

func csrfSign(secret []byte, value string) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(value))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func isCSRFSafeMethod(method string, safeMethods []string) bool {
	for _, item := range safeMethods {
		if strings.EqualFold(method, item) {
			return true
		}
	}
	return false
}

func defaultCSRFErrorHandler(c *gin.Context, err error) {
	c.JSON(http.StatusForbidden, gin.H{
		"code":    http.StatusForbidden,
		"message": "CSRF token invalid",
		"error":   fmt.Sprintf("%v", err),
	})
	c.Abort()
}
