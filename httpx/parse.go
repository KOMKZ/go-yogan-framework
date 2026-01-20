package httpx

import (
	"github.com/gin-gonic/gin"
)

// Parse automatically extract request parameters (query + body + path)
// Supports form/json/uri tags
func Parse(c *gin.Context, req interface{}) error {
	// Bind URI parameters (such as :id)
	if err := c.ShouldBindUri(req); err != nil {
		// URI parameter binding failed possibly due to the absence of a uri tag, continue
		_ = err
	}

	// 2. Bind Query parameters (form tag)
	if err := c.ShouldBindQuery(req); err != nil {
		// Query parameter binding failed possibly due to absence of form tag, continue
		_ = err
	}

	// 3. Bind Body parameters (json tag)
	// Only attempt to bind the body when Content-Length > 0
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(req); err != nil {
			return err
		}
	}

	return nil
}

