package permission

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestInferModule(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{path: "/api/admin/admins/page", want: "admins"},
		{path: "/api/admin/permission-hub/resources/page", want: "permission_hub"},
		{path: "/api/auth/login", want: "auth"},
		{path: "/health", want: "health"},
		{path: "", want: "unknown"},
	}

	for _, tt := range tests {
		if got := InferModule(tt.path); got != tt.want {
			t.Fatalf("InferModule(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestScanRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/api/admin/admins/page", func(c *gin.Context) { c.Status(200) })

	resources := ScanRoutes(engine)
	if len(resources) != 1 {
		t.Fatalf("expected 1 route, got %d", len(resources))
	}

	if resources[0].Module != "admins" {
		t.Fatalf("expected module admins, got %s", resources[0].Module)
	}
}
