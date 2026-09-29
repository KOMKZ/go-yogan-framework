package permission

import "context"

// Decision is the policy evaluation result.
type Decision int

const (
	Allow Decision = iota
	Deny
	NotConfigured
)

// RequestMeta contains normalized request metadata for policy evaluation.
type RequestMeta struct {
	Method string
	// Path 是路由匹配模板（如 gin FullPath，含 :param 段）；与参数化声明的策略直接精确匹配。
	Path string
	// ConcretePath 是真实请求路径（URL.Path）。声明策略按具体值展开（如同一路由按 scope
	// 注册多条具体策略）时，pattern 精确匹配会 miss，引擎用它兜底二次匹配（ticket 000277）。
	ConcretePath string
	UserID       int64
}

// PolicyEngine evaluates whether a request should be allowed.
type PolicyEngine interface {
	Evaluate(ctx context.Context, req RequestMeta) Decision
}

// RoutePolicy binds an API route template to a permission code.
type RoutePolicy struct {
	Method         string
	PathPattern    string
	PermissionCode string
}

// PolicyLoader loads active route policies from persistent storage.
type PolicyLoader func(ctx context.Context) ([]RoutePolicy, error)

// PermissionLoader loads a user's permission code set.
type PermissionLoader func(ctx context.Context, userID int64) ([]string, error)
