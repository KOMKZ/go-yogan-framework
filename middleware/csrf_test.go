package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCSRF_IssueAndValidate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := DefaultCSRFConfig([]byte("test-csrf-secret"))
	router := gin.New()
	router.GET("/csrf-token", func(c *gin.Context) {
		resp, err := IssueCSRFToken(c, cfg)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, resp)
	})
	router.POST("/protected", CSRF(cfg), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	tokenResp := httptest.NewRecorder()
	tokenReq := httptest.NewRequest(http.MethodGet, "/csrf-token", nil)
	router.ServeHTTP(tokenResp, tokenReq)
	if tokenResp.Code != http.StatusOK {
		t.Fatalf("issue token failed: status=%d body=%s", tokenResp.Code, tokenResp.Body.String())
	}
	token := extractTestCSRFToken(t, tokenResp.Body.String())
	cookies := tokenResp.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected csrf digest cookie")
	}

	req := httptest.NewRequest(http.MethodPost, "/protected", nil)
	req.Header.Set(cfg.HeaderName, token)
	req.AddCookie(cookies[0])
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusNoContent {
		t.Fatalf("expected csrf success, got status=%d body=%s", resp.Code, resp.Body.String())
	}
}

func TestCSRF_RejectsMissingToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := DefaultCSRFConfig([]byte("test-csrf-secret"))
	router := gin.New()
	router.POST("/protected", CSRF(cfg), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/protected", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden, got status=%d body=%s", resp.Code, resp.Body.String())
	}
}

func TestCSRF_SkipsSafeMethods(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := DefaultCSRFConfig([]byte("test-csrf-secret"))
	router := gin.New()
	router.GET("/protected", CSRF(cfg), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusNoContent {
		t.Fatalf("expected safe method skip, got status=%d body=%s", resp.Code, resp.Body.String())
	}
}

func TestCSRF_Skipper(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := DefaultCSRFConfig([]byte("test-csrf-secret"))
	cfg.Skipper = func(c *gin.Context) bool {
		return c.FullPath() == "/public"
	}
	router := gin.New()
	router.POST("/public", CSRF(cfg), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/public", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusNoContent {
		t.Fatalf("expected skipper success, got status=%d body=%s", resp.Code, resp.Body.String())
	}
}

func extractTestCSRFToken(t *testing.T, body string) string {
	t.Helper()
	const marker = `"csrf_token":"`
	start := strings.Index(body, marker)
	if start < 0 {
		t.Fatalf("csrf token missing in body: %s", body)
	}
	start += len(marker)
	end := strings.Index(body[start:], `"`)
	if end < 0 {
		t.Fatalf("csrf token not terminated in body: %s", body)
	}
	return body[start : start+end]
}
