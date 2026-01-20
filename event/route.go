package event

import (
	"sort"
	"strings"
	"sync"
)

// RouteConfig single route configuration
type RouteConfig struct {
	Driver string `mapstructure:"driver"` // "kafka" | "memory"
	Topic  string `mapstructure:"topic"`  // Kafka topic (mandatory when driver is set to kafka)
}

// Router event router
// Match route rules based on event name, supports wildcards
type Router struct {
	mu     sync.RWMutex
	routes map[string]RouteConfig // Exact route match
	sorted []routeEntry           // Sorted routes (prioritizing exact matches, then wildcards)
}

// routeEntry route entry (for sorting)
type routeEntry struct {
	pattern    string
	config     RouteConfig
	isWildcard bool
	priority   int // Priority: exact match > more specific wildcard > general wildcard
}

// Create router
func NewRouter() *Router {
	return &Router{
		routes: make(map[string]RouteConfig),
	}
}

// LoadRoutes load route configuration
func (r *Router) LoadRoutes(routes map[string]RouteConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.routes = routes
	r.rebuildSortedRoutes()
}

// rebuildSortedRoutes Rebuild the sorted route list
func (r *Router) rebuildSortedRoutes() {
	r.sorted = make([]routeEntry, 0, len(r.routes))

	for pattern, config := range r.routes {
		entry := routeEntry{
			pattern:    pattern,
			config:     config,
			isWildcard: strings.Contains(pattern, "*"),
		}

		// Calculate priority (the smaller the number, the higher the priority)
		if !entry.isWildcard {
			entry.priority = 0 // Exact match highest priority
		} else if pattern == "*" {
			entry.priority = 1000 // General wildcard lowest priority
		} else {
			// Wildcard priority is determined by the length of the prefix
			// "order.created.*" takes precedence over "order.*"
			entry.priority = 100 - len(strings.TrimSuffix(pattern, "*"))
		}

		r.sorted = append(r.sorted, entry)
	}

	// Sort by priority
	sort.Slice(r.sorted, func(i, j int) bool {
		return r.sorted[i].priority < r.sorted[j].priority
	})
}

// Match event name, return route configuration
// If there is no matching route, return nil
// Note: Use ":" as a separator for event names in the configuration file (to avoid Viper interpreting "." as a nested path)
// So when matching, the "." in the event names needs to be converted to ":"
func (r *Router) Match(eventName string) *RouteConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Convert "." to ":" in event names for matching
	normalizedName := strings.ReplaceAll(eventName, ".", ":")

	// match by priority order
	for _, entry := range r.sorted {
		if r.matchPattern(entry.pattern, normalizedName) {
			config := entry.config
			return &config
		}
	}

	return nil
}

// matchPattern matching pattern
// Support:
// - Exact match: "order:created" matches "order:created"
// - Suffix wildcard: "order:*" matches "order:created", "order:updated"
// - General wildcard: "*" matches all events
// Note: Use ":" as a delimiter in the configuration file (to avoid Viper interpreting "." as a nested path)
func (r *Router) matchPattern(pattern, eventName string) bool {
	// Exact match
	if pattern == eventName {
		return true
	}

	// General wildcard
	if pattern == "*" {
		return true
	}

	// Supports suffix wildcard (format: "*")
	if strings.HasSuffix(pattern, ":*") {
		prefix := strings.TrimSuffix(pattern, ":*")
		return strings.HasPrefix(eventName, prefix+":")
	}

	// Compatibility with old format ".*"
	if strings.HasSuffix(pattern, ".*") {
		prefix := strings.TrimSuffix(pattern, ".*")
		return strings.HasPrefix(eventName, prefix+".")
	}

	// Single-level wildcard (e.g., "order:*:done")
	if strings.Contains(pattern, "*") {
		return r.matchWildcard(pattern, eventName)
	}

	return false
}

// matchWildcard wildcard matching
// treat * as matching any non-empty string
// Supports both ":" and "." as delimiter options
func (r *Router) matchWildcard(pattern, eventName string) bool {
	// Prefer using ":" as the delimiter
	separator := ":"
	if !strings.Contains(pattern, ":") && strings.Contains(pattern, ".") {
		separator = "."
	}

	patternParts := strings.Split(pattern, separator)
	eventParts := strings.Split(eventName, separator)

	if len(patternParts) != len(eventParts) {
		return false
	}

	for i, pp := range patternParts {
		if pp == "*" {
			continue
		}
		if pp != eventParts[i] {
			return false
		}
	}

	return true
}

// HasRoutes是否有路由配置
func (r *Router) HasRoutes() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.routes) > 0
}

// RouteCount route quantity
func (r *Router) RouteCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.routes)
}
