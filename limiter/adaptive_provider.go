package limiter

// AdaptiveProvider 自适应限流数据提供者（依赖注入）
//
// 使用说明：
//   - 实现此接口以提供系统负载数据（CPU、内存、系统负载）
//   - 如果未注入Provider，自适应限流将不生效，回退到固定限流
//   - 可以使用 gopsutil 等库实现具体的采集逻辑
//
// 示例实现：
//
//	type SystemMetricsProvider struct {
//	    // 可以集成 gopsutil 等库
//	}
//
//	func (p *SystemMetricsProvider) GetCPUUsage() float64 {
//	    // 实现CPU使用率采集
//	    return 0.65
//	}
type AdaptiveProvider interface {
	// GetCPUUsage 获取CPU使用率（0.0-1.0）
	GetCPUUsage() float64

	// GetMemoryUsage 获取内存使用率（0.0-1.0）
	GetMemoryUsage() float64

	// GetSystemLoad 获取系统负载（通常为load average / CPU核数）
	GetSystemLoad() float64
}

