package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/KOMKZ/go-yogan-framework/jwt"
	"github.com/KOMKZ/go-yogan-framework/permission"
	"github.com/gin-gonic/gin"
)

type stubPolicyEngine struct {
	decision permission.Decision
}

func (s stubPolicyEngine) Evaluate(context.Context, permission.RequestMeta) permission.Decision {
	return s.decision
}

type stubTokenManager struct {
	claims *jwt.Claims
	err    error
}

func (s stubTokenManager) GenerateAccessToken(context.Context, string, map[string]interface{}) (string, error) {
	return "", nil
}

func (s stubTokenManager) GenerateRefreshToken(context.Context, string) (string, error) {
	return "", nil
}

func (s stubTokenManager) VerifyToken(context.Context, string) (*jwt.Claims, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.claims, nil
}

func (s stubTokenManager) RefreshToken(context.Context, string) (string, error) {
	return "", nil
}

func (s stubTokenManager) RevokeToken(context.Context, string) error {
	return nil
}

func (s stubTokenManager) RevokeUserTokens(context.Context, string) error {
	return nil
}

func TestPermissionGateDenyFromContextUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Set("user_id", int64(10))
		c.Next()
	})
	engine.Use(PermissionGate(PermissionGateConfig{
		PolicyEngine:  stubPolicyEngine{decision: permission.Deny},
		DefaultPolicy: permission.Allow,
	}))
	engine.GET("/api/admin/roles/page", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/admin/roles/page", nil)
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.Code)
	}
}

func TestPermissionGateDefaultPolicyForNotConfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Set("user_id", int64(10))
		c.Next()
	})
	engine.Use(PermissionGate(PermissionGateConfig{
		PolicyEngine:  stubPolicyEngine{decision: permission.NotConfigured},
		DefaultPolicy: permission.Allow,
	}))
	engine.GET("/api/admin/options/sidebar-menu", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/admin/options/sidebar-menu", nil)
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
}

func TestPermissionGateResolveUserFromToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(PermissionGate(PermissionGateConfig{
		PolicyEngine: stubPolicyEngine{decision: permission.Allow},
		TokenManager: stubTokenManager{claims: &jwt.Claims{UserID: 42}},
	}))
	engine.GET("/api/admin/admins/page", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/admin/admins/page", nil)
	req.Header.Set("Authorization", "Bearer token")
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
}

func TestPermissionGateSkipPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(PermissionGate(PermissionGateConfig{
		PolicyEngine:  stubPolicyEngine{decision: permission.Deny},
		DefaultPolicy: permission.Deny,
		SkipPaths:     []string{"/api/auth"},
		TokenManager:  stubTokenManager{err: errors.New("should not be called")},
	}))
	engine.POST("/api/auth/login", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
}
