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
	Path   string
	UserID int64
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
