package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/KOMKZ/go-yogan-framework/jwt"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// MockTokenManager 模拟 TokenManager
type MockTokenManager struct {
	claims *jwt.Claims
	err    error
}

func (m *MockTokenManager) GenerateAccessToken(ctx context.Context, subject string, claims map[string]interface{}) (string, error) {
	return "mock-access-token", nil
}

func (m *MockTokenManager) GenerateRefreshToken(ctx context.Context, subject string) (string, error) {
	return "mock-refresh-token", nil
}

func (m *MockTokenManager) VerifyToken(ctx context.Context, token string) (*jwt.Claims, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.claims, nil
}

func (m *MockTokenManager) RefreshToken(ctx context.Context, refreshToken string) (string, error) {
	return "new-access-token", nil
}

func (m *MockTokenManager) RevokeToken(ctx context.Context, token string) error {
	return nil
}

func (m *MockTokenManager) RevokeUserTokens(ctx context.Context, subject string) error {
	return nil
}

func TestJWT_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	mockManager := &MockTokenManager{
		claims: &jwt.Claims{
			UserID:   123,
			Username: "testuser",
			Roles:    []string{"admin", "user"},
		},
	}

	router.Use(JWT(mockManager))
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestJWT_MissingToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	mockManager := &MockTokenManager{}

	router.Use(JWT(mockManager))
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestJWT_InvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	mockManager := &MockTokenManager{
		err: errors.New("invalid token"),
	}

	router.Use(JWT(mockManager))
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestJWTWithConfig_Skipper(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	mockManager := &MockTokenManager{}

	config := JWTConfig{
		Skipper: func(c *gin.Context) bool {
			return c.Request.URL.Path == "/public"
		},
	}

	router.Use(JWTWithConfig(mockManager, config))
	router.GET("/public", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "public"})
	})

	req := httptest.NewRequest("GET", "/public", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestJWTWithConfig_QueryToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	mockManager := &MockTokenManager{
		claims: &jwt.Claims{
			UserID:   456,
			Username: "queryuser",
		},
	}

	config := JWTConfig{
		TokenLookup:   "query:token",
		TokenHeadName: "",
	}

	router.Use(JWTWithConfig(mockManager, config))
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/protected?token=valid-token", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestJWTWithConfig_CookieToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	mockManager := &MockTokenManager{
		claims: &jwt.Claims{
			UserID:   789,
			Username: "cookieuser",
		},
	}

	config := JWTConfig{
		TokenLookup:   "cookie:auth_token",
		TokenHeadName: "",
	}

	router.Use(JWTWithConfig(mockManager, config))
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: "valid-token"})
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestJWTWithConfig_InvalidLookup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	mockManager := &MockTokenManager{}

	config := JWTConfig{
		TokenLookup: "invalid-format",
	}

	router.Use(JWTWithConfig(mockManager, config))
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestJWTWithConfig_UnknownSource(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	mockManager := &MockTokenManager{}

	config := JWTConfig{
		TokenLookup: "unknown:field",
	}

	router.Use(JWTWithConfig(mockManager, config))
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestGetClaims(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	// 无 claims
	claims, ok := GetClaims(c)
	assert.False(t, ok)
	assert.Nil(t, claims)

	// 设置 claims
	expectedClaims := &jwt.Claims{UserID: 123, Username: "test"}
	c.Set("jwt_claims", expectedClaims)

	claims, ok = GetClaims(c)
	assert.True(t, ok)
	assert.Equal(t, int64(123), claims.UserID)
}

func TestGetUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	// 无 user_id
	userID, ok := GetUserID(c)
	assert.False(t, ok)
	assert.Equal(t, int64(0), userID)

	// 设置 user_id
	c.Set("user_id", int64(456))

	userID, ok = GetUserID(c)
	assert.True(t, ok)
	assert.Equal(t, int64(456), userID)
}

func TestGetUsername(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	// 无 username
	username, ok := GetUsername(c)
	assert.False(t, ok)
	assert.Equal(t, "", username)

	// 设置 username
	c.Set("username", "testuser")

	username, ok = GetUsername(c)
	assert.True(t, ok)
	assert.Equal(t, "testuser", username)
}

func TestHasRole(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	// 无 roles
	assert.False(t, HasRole(c, "admin"))

	// 设置错误类型
	c.Set("roles", "not-a-slice")
	assert.False(t, HasRole(c, "admin"))

	// 设置正确的 roles
	c.Set("roles", []string{"admin", "user"})
	assert.True(t, HasRole(c, "admin"))
	assert.True(t, HasRole(c, "user"))
	assert.False(t, HasRole(c, "superadmin"))
}

func TestJWTWithConfig_CustomErrorHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	mockManager := &MockTokenManager{
		err: errors.New("custom error"),
	}

	config := JWTConfig{
		ErrorHandler: func(c *gin.Context, err error) {
			c.JSON(http.StatusForbidden, gin.H{"custom": "error"})
			c.Abort()
		},
	}

	router.Use(JWTWithConfig(mockManager, config))
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusForbidden, resp.Code)
	assert.Contains(t, resp.Body.String(), "custom")
}
