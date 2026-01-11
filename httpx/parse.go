package httpx

import (
	"github.com/gin-gonic/gin"
)

// Parse 自动解析请求参数（query + body + path）
// 支持 form/json/uri tag
func Parse(c *gin.Context, req interface{}) error {
	// 1. 绑定 URI 参数（uri tag，如 :id）
	if err := c.ShouldBindUri(req); err != nil {
		// URI 参数绑定失败可能是因为没有 uri tag，继续
		_ = err
	}

	// 2. 绑定 Query 参数（form tag）
	if err := c.ShouldBindQuery(req); err != nil {
		// Query 参数绑定失败可能是因为没有 form tag，继续
		_ = err
	}

	// 3. 绑定 Body 参数（json tag）
	// 只有当 Content-Length > 0 时才尝试绑定 body
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(req); err != nil {
			return err
		}
	}

	return nil
}

