package permission

import (
	"context"
	"strings"
	"sync"
	"time"
)

const defaultUserPermissionTTL = 5 * time.Minute

type userPermEntry struct {
	permSet   map[string]bool
	expiresAt time.Time
}

// CachedPolicyEngine is an in-memory policy engine with route-policy and user-permission cache.
type CachedPolicyEngine struct {
	mu              sync.RWMutex
	routePolicies   map[string]string
	userPermissions map[int64]*userPermEntry

	policyLoader PolicyLoader
	permLoader   PermissionLoader
	ttl          time.Duration
}

func NewCachedPolicyEngine(
	policyLoader PolicyLoader,
	permLoader PermissionLoader,
	ttl time.Duration,
) *CachedPolicyEngine {
	if ttl <= 0 {
		ttl = defaultUserPermissionTTL
	}

	return &CachedPolicyEngine{
		routePolicies:   make(map[string]string),
		userPermissions: make(map[int64]*userPermEntry),
		policyLoader:    policyLoader,
		permLoader:      permLoader,
		ttl:             ttl,
	}
}

func (e *CachedPolicyEngine) Evaluate(ctx context.Context, req RequestMeta) Decision {
	requiredPerm := e.getRoutePermission(req.Method, req.Path)
	if requiredPerm == "" {
		return NotConfigured
	}

	userPerms, err := e.getUserPermissions(ctx, req.UserID)
	if err != nil {
		return Deny
	}

	if userPerms["*:*"] || userPerms[requiredPerm] {
		return Allow
	}

	return Deny
}

func (e *CachedPolicyEngine) getRoutePermission(method, path string) string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.routePolicies[buildRouteKey(method, path)]
}

func (e *CachedPolicyEngine) getUserPermissions(ctx context.Context, userID int64) (map[string]bool, error) {
	now := time.Now()

	e.mu.RLock()
	entry, ok := e.userPermissions[userID]
	e.mu.RUnlock()
	if ok && now.Before(entry.expiresAt) {
		return entry.permSet, nil
	}

	if e.permLoader == nil {
		return nil, context.Canceled
	}

	codes, err := e.permLoader(ctx, userID)
	if err != nil {
		return nil, err
	}

	permSet := make(map[string]bool, len(codes))
	for _, code := range codes {
		trimmed := strings.TrimSpace(code)
		if trimmed == "" {
			continue
		}
		permSet[trimmed] = true
	}

	e.mu.Lock()
	e.userPermissions[userID] = &userPermEntry{
		permSet:   permSet,
		expiresAt: now.Add(e.ttl),
	}
	e.mu.Unlock()

	return permSet, nil
}

// LoadPolicies reloads active route policies from policyLoader.
func (e *CachedPolicyEngine) LoadPolicies(ctx context.Context) error {
	if e.policyLoader == nil {
		e.mu.Lock()
		e.routePolicies = make(map[string]string)
		e.mu.Unlock()
		return nil
	}

	policies, err := e.policyLoader(ctx)
	if err != nil {
		return err
	}

	newMap := make(map[string]string, len(policies))
	for _, p := range policies {
		if strings.TrimSpace(p.PermissionCode) == "" {
			continue
		}
		key := buildRouteKey(p.Method, p.PathPattern)
		newMap[key] = strings.TrimSpace(p.PermissionCode)
	}

	e.mu.Lock()
	e.routePolicies = newMap
	e.mu.Unlock()

	return nil
}

func (e *CachedPolicyEngine) ReloadPolicies(ctx context.Context) error {
	return e.LoadPolicies(ctx)
}

func (e *CachedPolicyEngine) InvalidateUser(userID int64) {
	e.mu.Lock()
	delete(e.userPermissions, userID)
	e.mu.Unlock()
}

func (e *CachedPolicyEngine) InvalidateAllUsers() {
	e.mu.Lock()
	e.userPermissions = make(map[int64]*userPermEntry)
	e.mu.Unlock()
}

func buildRouteKey(method, path string) string {
	return strings.ToUpper(strings.TrimSpace(method)) + ":" + strings.TrimSpace(path)
}
