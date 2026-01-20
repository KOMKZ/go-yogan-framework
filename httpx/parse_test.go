package httpx

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TestParse_QueryParams test query parameter parsing
func TestParse_QueryParams(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type Request struct {
		Name string `form:"name"`
		Age  int    `form:"age"`
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/test?name=john&age=30", nil)

	var req Request
	err := Parse(c, &req)

	assert.NoError(t, err)
	assert.Equal(t, "john", req.Name)
	assert.Equal(t, 30, req.Age)
}

// TestParse_JSONBody test JSON body parsing
func TestParse_JSONBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type Request struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	body := `{"name":"john","email":"john@example.com"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/test", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.ContentLength = int64(len(body))

	var req Request
	err := Parse(c, &req)

	assert.NoError(t, err)
	assert.Equal(t, "john", req.Name)
	assert.Equal(t, "john@example.com", req.Email)
}

// TestParse_URIParams test URI parameter parsing
func TestParse_URIParams(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type Request struct {
		ID int `uri:"id"`
	}

	engine := gin.New()
	var req Request
	var parseErr error

	engine.GET("/users/:id", func(c *gin.Context) {
		parseErr = Parse(c, &req)
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("GET", "/users/123", nil)
	engine.ServeHTTP(w, httpReq)

	assert.NoError(t, parseErr)
	assert.Equal(t, 123, req.ID)
}

// TestParse_CombinedParams test combined parameter parsing
func TestParse_CombinedParams(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type Request struct {
		ID     int    `uri:"id"`
		Filter string `form:"filter"`
		Name   string `json:"name"`
	}

	engine := gin.New()
	var req Request
	var parseErr error

	engine.POST("/users/:id", func(c *gin.Context) {
		parseErr = Parse(c, &req)
		c.String(200, "ok")
	})

	body := `{"name":"john"}`
	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("POST", "/users/123?filter=active", bytes.NewBufferString(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.ContentLength = int64(len(body))
	engine.ServeHTTP(w, httpReq)

	assert.NoError(t, parseErr)
	assert.Equal(t, 123, req.ID)
	assert.Equal(t, "active", req.Filter)
	assert.Equal(t, "john", req.Name)
}

// TestParse_InvalidJSON test invalid JSON body
func TestParse_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type Request struct {
		Name string `json:"name"`
	}

	body := `{"name": invalid}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/test", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.ContentLength = int64(len(body))

	var req Request
	err := Parse(c, &req)

	assert.Error(t, err)
}

// TestParse_EmptyBody test empty Body
func TestParse_EmptyBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type Request struct {
		Name string `json:"name"`
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/test", nil)
	c.Request.ContentLength = 0

	var req Request
	err := Parse(c, &req)

	assert.NoError(t, err)
	assert.Empty(t, req.Name)
}
