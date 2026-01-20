package swagger

import (
	"net/http"

	"github.com/KOMKZ/go-yogan-framework/errcode"
)

// Module Code: 80 (Framework Layer Swagger Module)
const moduleCodeSwagger = 80

// Error code definitions
var (
	// ErrSwaggerDisabled Swagger is not enabled
	ErrSwaggerDisabled = errcode.Register(errcode.New(
		moduleCodeSwagger, 1, "swagger", "swagger.disabled", "Swagger is disabled", http.StatusServiceUnavailable,
	))

	// ErrSwaggerInitFailed Swagger initialization failed
	ErrSwaggerInitFailed = errcode.Register(errcode.New(
		moduleCodeSwagger, 2, "swagger", "swagger.init_failed", "Swagger initialization failed", http.StatusInternalServerError,
	))

	// ErrSwaggerDocNotFound Swagger document not found
	ErrSwaggerDocNotFound = errcode.Register(errcode.New(
		moduleCodeSwagger, 3, "swagger", "swagger.doc_not_found", "Swagger documentation not found", http.StatusNotFound,
	))
)
