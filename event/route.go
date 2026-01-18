package event

import (
	"sort"
	"strings"
	"sync"
)

// RouteConfig 单条路由配置
type RouteConfig struct {
	Driver string `mapstructure:"driver"` // "kafka" | "memory"
	Topic  string `mapstructure:"topic"`  // Kafka topic（driver=kafka 时必填）
}

// Router 事件路由器
// 根据事件名称匹配路由规则，支持通配符
type Router struct {
	mu     sync.RWMutex
	routes map[string]RouteConfig // 精确匹配路由
	sorted []routeEntry           // 排序后的路由（优先精确匹配，再通配符）
}

// routeEntry 路由条目（用于排序）
type routeEntry struct {
	pattern    string
	config     RouteConfig
	isWildcard bool
	priority   int // 优先级：精确匹配 > 更具体的通配符 > 通用通配符
}

// NewRouter 创建路由器
func NewRouter() *Router {
	return &Router{
		routes: make(map[string]RouteConfig),
	}
}

// LoadRoutes 加载路由配置
func (r *Router) LoadRoutes(routes map[string]RouteConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.routes = routes
	r.rebuildSortedRoutes()
}

// rebuildSortedRoutes 重建排序后的路由列表
func (r *Router) rebuildSortedRoutes() {
	r.sorted = make([]routeEntry, 0, len(r.routes))

	for pattern, config := range r.routes {
		entry := routeEntry{
			pattern:    pattern,
			config:     config,
			isWildcard: strings.Contains(pattern, "*"),
		}

		// 计算优先级（数字越小优先级越高）
		if !entry.isWildcard {
			entry.priority = 0 // 精确匹配最高优先级
		} else if pattern == "*" {
			entry.priority = 1000 // 通用通配符最低优先级
		} else {
			// 通配符优先级根据 prefix 长度确定
			// "order.created.*" 优先于 "order.*"
			entry.priority = 100 - len(strings.TrimSuffix(pattern, "*"))
		}

		r.sorted = append(r.sorted, entry)
	}

	// 按优先级排序
	sort.Slice(r.sorted, func(i, j int) bool {
		return r.sorted[i].priority < r.sorted[j].priority
	})
}

// Match 匹配事件名称，返回路由配置
// 如果没有匹配的路由，返回 nil
// 注意：配置文件中事件名称使用 ":" 作为分隔符（避免 Viper 将 "." 解析为嵌套路径）
// 所以匹配时需要将事件名称中的 "." 转换为 ":"
func (r *Router) Match(eventName string) *RouteConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 将事件名称中的 "." 转换为 ":" 进行匹配
	normalizedName := strings.ReplaceAll(eventName, ".", ":")

	// 按优先级顺序匹配
	for _, entry := range r.sorted {
		if r.matchPattern(entry.pattern, normalizedName) {
			config := entry.config
			return &config
		}
	}

	return nil
}

// matchPattern 匹配模式
// 支持：
// - 精确匹配："order:created" 匹配 "order:created"
// - 后缀通配符："order:*" 匹配 "order:created", "order:updated"
// - 通用通配符："*" 匹配所有事件
// 注意：配置文件中使用 ":" 作为分隔符（避免 Viper 将 "." 解析为嵌套路径）
func (r *Router) matchPattern(pattern, eventName string) bool {
	// 精确匹配
	if pattern == eventName {
		return true
	}

	// 通用通配符
	if pattern == "*" {
		return true
	}

	// 后缀通配符（支持 ":*" 格式）
	if strings.HasSuffix(pattern, ":*") {
		prefix := strings.TrimSuffix(pattern, ":*")
		return strings.HasPrefix(eventName, prefix+":")
	}

	// 兼容旧格式 ".*"
	if strings.HasSuffix(pattern, ".*") {
		prefix := strings.TrimSuffix(pattern, ".*")
		return strings.HasPrefix(eventName, prefix+".")
	}

	// 单层通配符（如 "order:*:done"）
	if strings.Contains(pattern, "*") {
		return r.matchWildcard(pattern, eventName)
	}

	return false
}

// matchWildcard 通配符匹配
// 将 * 视为匹配任意非空字符串
// 支持 ":" 和 "." 两种分隔符
func (r *Router) matchWildcard(pattern, eventName string) bool {
	// 优先使用 ":" 作为分隔符
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

// HasRoutes 是否有路由配置
func (r *Router) HasRoutes() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.routes) > 0
}

// RouteCount 路由数量
func (r *Router) RouteCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.routes)
}
