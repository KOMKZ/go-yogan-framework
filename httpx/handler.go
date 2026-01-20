package httpx

import (
	"github.com/KOMKZ/go-yogan-framework/validator"
	"github.com/gin-gonic/gin"
)

// HandlerFunc generic handler function signature
// Req: Request type (supported types: form/json/uri tag)
// Response type
type HandlerFunc[Req any, Resp any] func(c *gin.Context, req *Req) (*Resp, error)

// Wrap Packaging Handler, automatically handle parsing, validation, response
// Decouple business logic from HTTP details
func Wrap[Req any, Resp any](handler HandlerFunc[Req, Resp]) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Automatically parse request (query + body + path)
		var req Req
		if err := Parse(c, &req); err != nil {
			HandleError(c, err) // Use unified error handling
			return
		}

		// 2. Execute parameter validation (if the request object implements the Validatable interface)
		if validatableReq, ok := any(&req).(validator.Validatable); ok {
			if err := validator.ValidateRequest(validatableReq); err != nil {
				HandleError(c, err) // Return 1010 + field details
				return
			}
		}

		// 3. Call business logic (pure functions, easy to test)
		resp, err := handler(c, &req)
		if err != nil {
			// ✅ Use intelligent error handling (automatically identify 404/400/500)
			HandleError(c, err)
			return
		}

		// 4. Automatically return response
		OkJson(c, resp)
	}
}

