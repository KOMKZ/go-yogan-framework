package swagger

import (
	"net/http"

	"github.com/KOMKZ/go-yogan-framework/errcode"
)

// 模块码：80（框架层 Swagger 模块）
const moduleCodeSwagger = 80

// 错误码定义
var (
	// ErrSwaggerDisabled Swagger 未启用
	ErrSwaggerDisabled = errcode.Register(errcode.New(
		moduleCodeSwagger, 1, "swagger", "swagger.disabled", "Swagger is disabled", http.StatusServiceUnavailable,
	))

	// ErrSwaggerInitFailed Swagger 初始化失败
	ErrSwaggerInitFailed = errcode.Register(errcode.New(
		moduleCodeSwagger, 2, "swagger", "swagger.init_failed", "Swagger initialization failed", http.StatusInternalServerError,
	))

	// ErrSwaggerDocNotFound Swagger 文档未找到
	ErrSwaggerDocNotFound = errcode.Register(errcode.New(
		moduleCodeSwagger, 3, "swagger", "swagger.doc_not_found", "Swagger documentation not found", http.StatusNotFound,
	))
)
