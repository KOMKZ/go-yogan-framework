// Package component 提供组件接口定义
// 这是最底层的包，不依赖任何业务包，避免循环依赖
package component

import "context"

// Component 组件接口（统一生命周期管理）
//
// 所有组件（核心组件、业务组件）都必须实现此接口
// 组件生命周期：Init → Start → Stop
type Component interface {
	// Name 组件名称（唯一标识）
	// 用于依赖声明和组件查找
	Name() string

	// DependsOn 声明依赖的组件名称
	// 注册中心会根据依赖关系进行拓扑排序，确定初始化顺序
	//
	// 支持可选依赖：
	//   - 强制依赖：直接返回组件名，如 "config", "logger"
	//   - 可选依赖：使用 "optional:" 前缀，如 "optional:telemetry"
	//
	// 示例：
	//   return []string{
	//       "config",              // 强制依赖：未注册会报错
	//       "logger",              // 强制依赖：未注册会报错
	//       "optional:telemetry",  // 可选依赖：未注册会跳过
	//   }
	//
	// 注意：
	//   - 可选依赖不影响初始化顺序（如果存在）
	//   - 可选依赖未注册时，组件需自行处理（如使用默认值）
	DependsOn() []string

	// Init 初始化组件（创建资源，不启动对外服务）
	//
	// 职责：
	// - 从 loader 读取配置
	// - 创建资源（连接池、客户端等）
	// - 不启动监听端口或对外服务
	//
	// 参数：
	//   ctx: 上下文
	//   loader: 配置加载器（组件直接读取配置，无需依赖 Registry）
	//
	// 注意：组件应该自己读取配置，而不是从 Registry 获取其他组件
	Init(ctx context.Context, loader ConfigLoader) error

	// Start 启动组件（对外提供服务或开始监听）
	//
	// 职责：
	// - 启动 HTTP Server、gRPC Server
	// - 连接数据库、Redis 等外部服务
	// - 开始监听消息队列
	Start(ctx context.Context) error

	// Stop 停止组件（释放资源，允许重复调用）
	//
	// 职责：
	// - 关闭连接
	// - 释放资源
	// - 保证幂等性（允许多次调用）
	Stop(ctx context.Context) error
}

// HealthChecker 健康检查接口
// 组件可选实现此接口，提供健康检查能力
type HealthChecker interface {
	// Check 执行健康检查
	// 返回 nil 表示健康，返回 error 表示不健康
	Check(ctx context.Context) error

	// Name 返回检查项名称（如 "database", "redis"）
	Name() string
}

// HealthCheckProvider 健康检查提供者接口
// 组件可选实现此接口，提供健康检查器
type HealthCheckProvider interface {
	GetHealthChecker() HealthChecker
}

// Registry 组件注册中心接口
//
// 职责：
// - 注册和管理组件
// - 解析组件依赖关系
// - 按依赖顺序执行组件生命周期方法
type Registry interface {
	// Register 注册组件
	//
	// 参数：
	//   comp: 组件实例
	//
	// 返回：
	//   error: 组件已存在或名称为空时返回错误
	Register(comp Component) error

	// Get 获取组件
	//
	// 参数：
	//   name: 组件名称
	//
	// 返回：
	//   Component: 组件实例
	//   bool: 组件是否存在
	Get(name string) (Component, bool)

	// MustGet 获取组件（不存在则 panic）
	//
	// 参数：
	//   name: 组件名称
	//
	// 返回：
	//   Component: 组件实例
	//
	// Panic:
	//   组件不存在时 panic
	MustGet(name string) Component

	// Has 检查组件是否已注册
	//
	// 参数：
	//   name: 组件名称
	//
	// 返回：
	//   bool: 组件是否已注册
	Has(name string) bool

	// Resolve 返回拓扑排序后的组件列表
	//
	// 返回：
	//   []Component: 按依赖顺序排列的组件列表
	//   error: 检测到循环依赖或依赖组件未注册时返回错误
	Resolve() ([]Component, error)

	// Init 按依赖顺序初始化所有组件
	//
	// 执行顺序：
	//   Layer 0 (无依赖组件) → Layer 1 → Layer 2 → ...
	//   同层级组件并发执行
	Init(ctx context.Context) error

	// Start 按依赖顺序启动所有组件
	//
	// 执行顺序：
	//   Layer 0 (无依赖组件) → Layer 1 → Layer 2 → ...
	//   同层级组件并发执行
	Start(ctx context.Context) error

	// Stop 反向顺序停止所有组件
	//
	// 执行顺序（反向）：
	//   ... → Layer 2 → Layer 1 → Layer 0
	//   同层级组件并发执行
	//   忽略 Stop 错误，确保所有组件都尝试停止
	Stop(ctx context.Context) error
}
