package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/KOMKZ/go-yogan-framework/errcode"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TestWrap_Success test successful response
func TestWrap_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type Request struct {
		Name string `json:"name"`
	}
	type Response struct {
		Greeting string `json:"greeting"`
	}

	handler := func(c *gin.Context, req *Request) (*Response, error) {
		return &Response{Greeting: "Hello, " + req.Name}, nil
	}

	engine := gin.New()
	engine.POST("/greet", Wrap(handler))

	body := `{"name":"World"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/greet", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = int64(len(body))
	engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Code int      `json:"code"`
		Msg  string   `json:"msg"`
		Data Response `json:"data"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	assert.Equal(t, "Hello, World", resp.Data.Greeting)
}

// TestParseError test parsing errors
func TestWrap_ParseError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type Request struct {
		Count int `json:"count"`
	}
	type Response struct{}

	handler := func(c *gin.Context, req *Request) (*Response, error) {
		return &Response{}, nil
	}

	engine := gin.New()
	engine.POST("/test", Wrap(handler))

	// Send invalid JSON
	body := `{"count": "not a number"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/test", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = int64(len(body))
	engine.ServeHTTP(w, req)

	// Should return an error
	assert.NotEqual(t, http.StatusOK, w.Code)
}

// TestWrap_BusinessError test business error
func TestWrap_BusinessError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type Request struct{}
	type Response struct{}

	handler := func(c *gin.Context, req *Request) (*Response, error) {
		return nil, errcode.New(10, 1, "test", "test.error", "业务错误", http.StatusBadRequest)
	}

	engine := gin.New()
	engine.POST("/test", Wrap(handler))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/test", nil)
	engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 100001, resp.Code) // 10*10000 + 1 = 100001
	assert.Equal(t, "业务错误", resp.Msg)
}

// TestWrap_UnknownError test for unknown error
func TestWrap_UnknownError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type Request struct{}
	type Response struct{}

	handler := func(c *gin.Context, req *Request) (*Response, error) {
		return nil, errors.New("unknown error")
	}

	engine := gin.New()
	engine.POST("/test", Wrap(handler))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/test", nil)
	engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 500, resp.Code)
}

// TestWrap_QueryParams test query parameters
func TestWrap_QueryParams(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type Request struct {
		Page     int `form:"page"`
		PageSize int `form:"page_size"`
	}
	type Response struct {
		Page     int `json:"page"`
		PageSize int `json:"page_size"`
	}

	handler := func(c *gin.Context, req *Request) (*Response, error) {
		return &Response{Page: req.Page, PageSize: req.PageSize}, nil
	}

	engine := gin.New()
	engine.GET("/list", Wrap(handler))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/list?page=2&page_size=20", nil)
	engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Code int      `json:"code"`
		Data Response `json:"data"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	assert.Equal(t, 2, resp.Data.Page)
	assert.Equal(t, 20, resp.Data.PageSize)
}

// TestWrap_URIParams test URI parameters
func TestWrap_URIParams(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type Request struct {
		ID int `uri:"id"`
	}
	type Response struct {
		ID int `json:"id"`
	}

	handler := func(c *gin.Context, req *Request) (*Response, error) {
		return &Response{ID: req.ID}, nil
	}

	engine := gin.New()
	engine.GET("/users/:id", Wrap(handler))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/users/123", nil)
	engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Code int      `json:"code"`
		Data Response `json:"data"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	assert.Equal(t, 123, resp.Data.ID)
}

// TestWrap_NilResponse test for nil response
func TestWrap_NilResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type Request struct{}
	type Response struct{}

	handler := func(c *gin.Context, req *Request) (*Response, error) {
		return nil, nil
	}

	engine := gin.New()
	engine.POST("/test", Wrap(handler))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/test", nil)
	engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
}
