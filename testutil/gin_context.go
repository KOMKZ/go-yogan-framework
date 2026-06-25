package testutil

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
)

// NewGinContext creates a gin test context for handler-level unit tests.
func NewGinContext(method, target string, body io.Reader) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)

	if method == "" {
		method = http.MethodGet
	}
	if target == "" {
		target = "/"
	}
	if body == nil {
		body = bytes.NewReader(nil)
	}

	writer := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(writer)
	ctx.Request = httptest.NewRequest(method, target, body)

	return ctx, writer
}

// SetUserID stores user_id in gin context, compatible with middleware.GetUserID.
func SetUserID(ctx *gin.Context, userID int64) {
	ctx.Set("user_id", userID)
}
