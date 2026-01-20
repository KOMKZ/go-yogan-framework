package limiter

// AdaptiveProvider adaptive rate limiting data provider (dependency injection)
//
// Usage instructions:
// - Implement this interface to provide system load data (CPU, memory, system load)
// - If Provider is not injected, adaptive rate limiting will not be effective, falling back to fixed rate limiting
// - Specific collection logic can be implemented using libraries such as gopsutil
//
// Example implementation:
//
//	type SystemMetricsProvider struct {
// // Can integrate libraries like gopsutil
//	}
//
//	func (p *SystemMetricsProvider) GetCPUUsage() float64 {
// // Implement CPU usage collection
//	    return 0.65
//	}
type AdaptiveProvider interface {
	// GetCPUUsage Gets CPU usage (0.0-1.0)
	GetCPUUsage() float64

	// GetMemoryUsage Gets memory usage (0.0-1.0)
	GetMemoryUsage() float64

	// GetSystemLoad Obtain system load (usually load average per CPU core)
	GetSystemLoad() float64
}

