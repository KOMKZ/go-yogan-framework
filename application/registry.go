package application

import (
	"github.com/KOMKZ/go-yogan-framework/registry"
)

// NewRegistry 创建组件注册中心
// 返回具体类型 *registry.Registry（可向上转型为 component.Registry）
func NewRegistry() *registry.Registry {
	return registry.NewRegistry()
}
