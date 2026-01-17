package component

// 组件名称常量
const (
	ComponentConfig     = "config"
	ComponentLogger     = "logger"
	ComponentDatabase   = "database"
	ComponentRedis      = "redis"
	ComponentGRPC       = "grpc"
	ComponentETCD       = "etcd"
	ComponentHTTPServer = "http_server"
	ComponentGovernance = "governance" // 🎯 治理组件
	ComponentLimiter    = "limiter"    // 🎯 限流组件
	ComponentTelemetry  = "telemetry"  // 🎯 遥测组件
	ComponentHealth     = "health"     // 🎯 健康检查组件
	ComponentJWT        = "jwt"        // 🎯 JWT 认证组件
	ComponentAuth       = "auth"       // 🎯 认证组件
	ComponentKafka      = "kafka"      // 🎯 Kafka 消息队列组件
	ComponentEvent      = "event"      // 🎯 事件分发组件
	ComponentCache      = "cache"      // 🎯 缓存编排组件
)
