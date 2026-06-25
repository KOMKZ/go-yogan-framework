package testutil

import (
	"net/http"
	"testing"

	"github.com/KOMKZ/go-yogan-framework/middleware"
)

func TestNewGinContext_Defaults(t *testing.T) {
	ctx, _ := NewGinContext("", "", nil)

	if ctx.Request.Method != http.MethodGet {
		t.Fatalf("unexpected method: %s", ctx.Request.Method)
	}
	if ctx.Request.URL.Path != "/" {
		t.Fatalf("unexpected path: %s", ctx.Request.URL.Path)
	}
}

func TestSetUserID(t *testing.T) {
	ctx, _ := NewGinContext(http.MethodGet, "/", nil)
	SetUserID(ctx, 101)

	userID, ok := middleware.GetUserID(ctx)
	if !ok {
		t.Fatal("expected user_id to exist")
	}
	if userID != 101 {
		t.Fatalf("unexpected user_id: %d", userID)
	}
}
