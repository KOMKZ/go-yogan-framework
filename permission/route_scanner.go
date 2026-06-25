package permission

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// APIResource is a normalized route ledger item discovered from gin route table.
type APIResource struct {
	Method      string
	Path        string
	HandlerName string
	Module      string
}

// ScanRoutes scans all routes from gin and returns normalized API resources.
func ScanRoutes(engine *gin.Engine) []APIResource {
	routes := engine.Routes()
	resources := make([]APIResource, 0, len(routes))

	for _, r := range routes {
		resources = append(resources, APIResource{
			Method:      r.Method,
			Path:        r.Path,
			HandlerName: r.Handler,
			Module:      InferModule(r.Path),
		})
	}

	return resources
}

// InferModule infers module name from route path.
func InferModule(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "unknown"
	}

	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) == 0 {
		return "unknown"
	}

	if parts[0] != "api" {
		return normalizeModule(parts[0])
	}

	if len(parts) < 2 {
		return "api"
	}

	if parts[1] == "admin" {
		if len(parts) >= 3 {
			return normalizeModule(parts[2])
		}
		return "admin"
	}

	return normalizeModule(parts[1])
}

func normalizeModule(segment string) string {
	value := strings.TrimSpace(segment)
	if value == "" {
		return "unknown"
	}
	return strings.ReplaceAll(value, "-", "_")
}
