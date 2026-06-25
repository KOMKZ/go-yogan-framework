package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/KOMKZ/go-yogan-framework/errcode"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestWrapStream_ParseAndValidate_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type Req struct {
		ID uint `uri:"id" binding:"required"`
	}

	handler := func(c *gin.Context, req *Req) error {
		sw := NewSSEWriter(c)
		_ = sw.WriteData(fmt.Sprintf("id=%d", req.ID))
		sw.WriteDone()
		return nil
	}

	engine := gin.New()
	engine.POST("/agents/:id/test-run", WrapStream(handler))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/agents/42/test-run", nil)
	engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "id=42")
	assert.Contains(t, w.Body.String(), "data: [DONE]")
}

func TestWrapStream_ParseError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type Req struct {
		Count int `json:"count"`
	}

	handler := func(c *gin.Context, req *Req) error {
		return nil
	}

	engine := gin.New()
	engine.POST("/test", WrapStream(handler))

	body := `{"count":"not_a_number"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/test", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = int64(len(body))
	engine.ServeHTTP(w, req)

	assert.NotEqual(t, http.StatusOK, w.Code)
}

type validatableStreamReq struct {
	Name string `json:"name"`
}

func (r *validatableStreamReq) Validate() error {
	if r.Name == "" {
		return errors.New("name is required")
	}
	return nil
}

func TestWrapStream_ValidateError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := func(c *gin.Context, req *validatableStreamReq) error {
		return nil
	}

	engine := gin.New()
	engine.POST("/test", WrapStream(handler))

	body := `{"name":""}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/test", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = int64(len(body))
	engine.ServeHTTP(w, req)

	assert.NotEqual(t, http.StatusOK, w.Code)
}

func TestWrapStream_HandlerError_BeforeWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type Req struct{}

	handler := func(c *gin.Context, req *Req) error {
		return errcode.New(10, 1, "test", "test.error", "业务错误", http.StatusBadRequest)
	}

	engine := gin.New()
	engine.POST("/test", WrapStream(handler))

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
	assert.Equal(t, 100001, resp.Code)
	assert.Equal(t, "业务错误", resp.Msg)
}

func TestWrapStream_HandlerError_AfterWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type Req struct{}

	handler := func(c *gin.Context, req *Req) error {
		sw := NewSSEWriter(c)
		_ = sw.WriteData("partial")
		return errors.New("mid-stream error")
	}

	engine := gin.New()
	engine.POST("/test", WrapStream(handler))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/test", nil)
	engine.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Contains(t, body, "data: partial")
}

func TestWrapStream_URIAndBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type Req struct {
		ID      uint   `uri:"id" binding:"required"`
		Message string `json:"message"`
	}

	handler := func(c *gin.Context, req *Req) error {
		sw := NewSSEWriter(c)
		_ = sw.WriteData(fmt.Sprintf("id=%d msg=%s", req.ID, req.Message))
		sw.WriteDone()
		return nil
	}

	engine := gin.New()
	engine.POST("/agents/:id/run", WrapStream(handler))

	body := `{"message":"hello"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/agents/7/run", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = int64(len(body))
	engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "id=7 msg=hello")
}

func TestWrapStream_NilError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type Req struct{}

	handler := func(c *gin.Context, req *Req) error {
		return nil
	}

	engine := gin.New()
	engine.POST("/test", WrapStream(handler))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/test", nil)
	engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
