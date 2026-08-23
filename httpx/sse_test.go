package httpx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestSSEWriter_NewSSEWriter_SetsHeaders(t *testing.T) {
	engine := gin.New()
	engine.GET("/sse", func(c *gin.Context) {
		NewSSEWriter(c)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/sse", nil)
	engine.ServeHTTP(w, req)

	assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache", w.Header().Get("Cache-Control"))
	assert.Equal(t, "keep-alive", w.Header().Get("Connection"))
	assert.Equal(t, "no", w.Header().Get("X-Accel-Buffering"))
}

func TestSSEWriter_WriteEvent(t *testing.T) {
	engine := gin.New()
	engine.GET("/sse", func(c *gin.Context) {
		sw := NewSSEWriter(c)
		type Payload struct {
			Text string `json:"text"`
		}
		err := sw.WriteEvent("message", Payload{Text: "hello"})
		assert.NoError(t, err)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/sse", nil)
	engine.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Contains(t, body, "event: message\n")
	assert.Contains(t, body, `data: {"text":"hello"}`)
	assert.True(t, strings.HasSuffix(body, "\n\n"))
}

func TestSSEWriter_WriteData(t *testing.T) {
	engine := gin.New()
	engine.GET("/sse", func(c *gin.Context) {
		sw := NewSSEWriter(c)
		err := sw.WriteData(`{"token":"abc"}`)
		assert.NoError(t, err)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/sse", nil)
	engine.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Contains(t, body, `data: {"token":"abc"}`)
	assert.True(t, strings.HasSuffix(body, "\n\n"))
}

func TestSSEWriter_WriteDone(t *testing.T) {
	engine := gin.New()
	engine.GET("/sse", func(c *gin.Context) {
		sw := NewSSEWriter(c)
		sw.WriteDone()
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/sse", nil)
	engine.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Contains(t, body, "data: [DONE]\n\n")
}

func TestSSEWriter_WriteError(t *testing.T) {
	engine := gin.New()
	engine.GET("/sse", func(c *gin.Context) {
		c.Set("trace_id", "sse-trace-1")
		sw := NewSSEWriter(c)
		sw.WriteError(assert.AnError)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/sse", nil)
	engine.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Contains(t, body, "event: error\n")
	assert.Contains(t, body, `"code":500`)
	assert.Contains(t, body, `"trace_id":"sse-trace-1"`)
	assert.Contains(t, body, assert.AnError.Error())
}

func TestSSEWriter_MultipleEvents(t *testing.T) {
	engine := gin.New()
	engine.GET("/sse", func(c *gin.Context) {
		sw := NewSSEWriter(c)
		_ = sw.WriteData("chunk-1")
		_ = sw.WriteData("chunk-2")
		sw.WriteDone()
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/sse", nil)
	engine.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Contains(t, body, "data: chunk-1\n\n")
	assert.Contains(t, body, "data: chunk-2\n\n")
	assert.Contains(t, body, "data: [DONE]\n\n")
}

func TestSSEWriter_Writer_ReturnsUnderlying(t *testing.T) {
	engine := gin.New()
	engine.GET("/sse", func(c *gin.Context) {
		sw := NewSSEWriter(c)
		assert.NotNil(t, sw.Writer())
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/sse", nil)
	engine.ServeHTTP(w, req)
}
