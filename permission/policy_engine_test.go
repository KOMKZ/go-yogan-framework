package permission

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCachedPolicyEngineEvaluate(t *testing.T) {
	engine := NewCachedPolicyEngine(
		func(context.Context) ([]RoutePolicy, error) {
			return []RoutePolicy{
				{Method: "GET", PathPattern: "/api/admin/admins/page", PermissionCode: "admin:read"},
			}, nil
		},
		func(_ context.Context, userID int64) ([]string, error) {
			if userID == 1 {
				return []string{"admin:read"}, nil
			}
			if userID == 2 {
				return []string{"*:*"}, nil
			}
			return []string{"admin:write"}, nil
		},
		time.Minute,
	)

	if err := engine.LoadPolicies(context.Background()); err != nil {
		t.Fatalf("LoadPolicies failed: %v", err)
	}

	if got := engine.Evaluate(context.Background(), RequestMeta{Method: "GET", Path: "/api/admin/admins/page", UserID: 1}); got != Allow {
		t.Fatalf("expected Allow for read user, got %v", got)
	}

	if got := engine.Evaluate(context.Background(), RequestMeta{Method: "GET", Path: "/api/admin/admins/page", UserID: 2}); got != Allow {
		t.Fatalf("expected Allow for wildcard user, got %v", got)
	}

	if got := engine.Evaluate(context.Background(), RequestMeta{Method: "GET", Path: "/api/admin/admins/page", UserID: 3}); got != Deny {
		t.Fatalf("expected Deny for missing permission, got %v", got)
	}

	if got := engine.Evaluate(context.Background(), RequestMeta{Method: "POST", Path: "/api/admin/admins", UserID: 1}); got != NotConfigured {
		t.Fatalf("expected NotConfigured for unknown route, got %v", got)
	}
}

func TestCachedPolicyEngineDenyOnPermissionLoaderError(t *testing.T) {
	engine := NewCachedPolicyEngine(
		func(context.Context) ([]RoutePolicy, error) {
			return []RoutePolicy{{Method: "GET", PathPattern: "/api/admin/roles/page", PermissionCode: "rbac:read"}}, nil
		},
		func(context.Context, int64) ([]string, error) {
			return nil, errors.New("load permission failed")
		},
		time.Minute,
	)

	if err := engine.LoadPolicies(context.Background()); err != nil {
		t.Fatalf("LoadPolicies failed: %v", err)
	}

	if got := engine.Evaluate(context.Background(), RequestMeta{Method: "GET", Path: "/api/admin/roles/page", UserID: 9}); got != Deny {
		t.Fatalf("expected Deny when permission loader fails, got %v", got)
	}
}
