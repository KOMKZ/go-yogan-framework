package httpx

import (
	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/KOMKZ/go-yogan-framework/validator"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// StreamHandlerFunc is the handler signature for streaming endpoints (e.g. SSE).
// Unlike HandlerFunc, it returns only an error because the response is written
// directly to the underlying ResponseWriter by the handler itself.
type StreamHandlerFunc[Req any] func(c *gin.Context, req *Req) error

// WrapStream wraps a streaming handler with the same Parse + Validate pipeline as Wrap,
// but delegates the actual response writing to the handler.
//
// Error handling strategy:
//   - If Parse or Validate fails (before handler is called): uses HandleError (JSON).
//   - If the handler returns an error and no bytes have been written yet: uses HandleError (JSON).
//   - If the handler returns an error after headers/body are already written: logs the error
//     because the HTTP status is already committed and cannot be changed.
func WrapStream[Req any](handler StreamHandlerFunc[Req]) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req Req
		if err := Parse(c, &req); err != nil {
			HandleError(c, err)
			return
		}

		if v, ok := any(&req).(validator.Validatable); ok {
			if err := validator.ValidateRequest(v); err != nil {
				HandleError(c, err)
				return
			}
		}

		if err := handler(c, &req); err != nil {
			if c.Writer.Written() {
				logger.ErrorCtx(c.Request.Context(), "httpx",
					"stream handler error after headers sent",
					zap.Error(err),
					zap.String("path", c.Request.URL.Path),
				)
				return
			}
			HandleError(c, err)
		}
	}
}
