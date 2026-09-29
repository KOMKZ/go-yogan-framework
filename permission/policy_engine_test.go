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

// 两级匹配（ticket 000277）：路由模板（含 :param）精确 miss 后，按真实请求路径
// 兜底命中"按具体值展开"的声明策略；pattern 命中时不做兜底，保持参数化声明语义。
func TestCachedPolicyEngineConcretePathFallback(t *testing.T) {
	engine := NewCachedPolicyEngine(
		func(context.Context) ([]RoutePolicy, error) {
			return []RoutePolicy{
				{Method: "POST", PathPattern: "/api/categories/bgm", PermissionCode: "category:bgm:write"},
				{Method: "GET", PathPattern: "/api/items/:id", PermissionCode: "item:read"},
			}, nil
		},
		func(_ context.Context, userID int64) ([]string, error) {
			if userID == 1 {
				return []string{"category:bgm:write", "item:read"}, nil
			}
			return []string{"item:read"}, nil
		},
		time.Minute,
	)
	if err := engine.LoadPolicies(context.Background()); err != nil {
		t.Fatalf("LoadPolicies failed: %v", err)
	}
	ctx := context.Background()

	// 声明为具体路径、请求模板含 :param：pattern miss 后由 ConcretePath 命中。
	concrete := RequestMeta{
		Method: "POST", Path: "/api/categories/:resource", ConcretePath: "/api/categories/bgm", UserID: 1,
	}
	if got := engine.Evaluate(ctx, concrete); got != Allow {
		t.Fatalf("expected Allow via concrete fallback, got %v", got)
	}
	// 同一路由、未授权用户：兜底命中后必须 Deny（修复前该请求 NotConfigured 被放行）。
	if got := engine.Evaluate(ctx, RequestMeta{Method: "POST", Path: "/api/categories/:resource", ConcretePath: "/api/categories/bgm", UserID: 2}); got != Deny {
		t.Fatalf("expected Deny for unauthorized user via concrete fallback, got %v", got)
	}
	// 具体路径未声明（未知 scope）：仍为 NotConfigured，保持 fail-open 边界不变。
	if got := engine.Evaluate(ctx, RequestMeta{Method: "POST", Path: "/api/categories/:resource", ConcretePath: "/api/categories/unknown", UserID: 2}); got != NotConfigured {
		t.Fatalf("expected NotConfigured for undeclared concrete path, got %v", got)
	}
	// 参数化声明优先：pattern 已命中时不再看 ConcretePath。
	if got := engine.Evaluate(ctx, RequestMeta{Method: "GET", Path: "/api/items/:id", ConcretePath: "/api/items/9", UserID: 2}); got != Allow {
		t.Fatalf("expected Allow for param pattern match, got %v", got)
	}
	if got := engine.Evaluate(ctx, RequestMeta{Method: "GET", Path: "/api/items/:id", ConcretePath: "/api/items/9", UserID: 1}); got != Allow {
		t.Fatalf("expected Allow for param pattern match with perm, got %v", got)
	}
	if got := engine.Evaluate(ctx, RequestMeta{Method: "DELETE", Path: "/api/items/:id", ConcretePath: "/api/items/9", UserID: 1}); got != NotConfigured {
		t.Fatalf("expected NotConfigured when method differs, got %v", got)
	}
}
