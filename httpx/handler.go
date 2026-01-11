package httpx

import (
	"github.com/KOMKZ/go-yogan-framework/validator"
	"github.com/gin-gonic/gin"
)

// HandlerFunc 泛型 Handler 函数签名
// Req: 请求类型（支持 form/json/uri tag）
// Resp: 响应类型
type HandlerFunc[Req any, Resp any] func(c *gin.Context, req *Req) (*Resp, error)

// Wrap 包装 Handler，自动处理解析、验证、响应
// 将业务逻辑从 HTTP 细节中解耦
func Wrap[Req any, Resp any](handler HandlerFunc[Req, Resp]) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 自动解析请求（query + body + path）
		var req Req
		if err := Parse(c, &req); err != nil {
			HandleError(c, err) // 使用统一错误处理
			return
		}

		// 2. 执行参数校验（如果请求对象实现了 Validatable 接口）
		if validatableReq, ok := any(&req).(validator.Validatable); ok {
			if err := validator.ValidateRequest(validatableReq); err != nil {
				HandleError(c, err) // 返回 1010 + 字段详情
				return
			}
		}

		// 3. 调用业务逻辑（纯函数，易于测试）
		resp, err := handler(c, &req)
		if err != nil {
			// ✅ 使用智能错误处理（自动识别 404/400/500）
			HandleError(c, err)
			return
		}

		// 4. 自动返回响应
		OkJson(c, resp)
	}
}

