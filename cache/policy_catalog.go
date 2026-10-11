// 本文件定义业务缓存策略目录（PolicyCatalog）：按 domain/scenario 两级声明缓存
// 场景，在构造时合并"全局/领域/场景"三级开关并校验 TTL，供应用启动时一次性
// 解析配置后冻结使用。
//
// 上游：各 app 的配置 provider 负责从自身 YAML 反序列化 PolicyDefinition 并调用
// NewPolicyCatalog；framework 不知道任何业务配置根 key（如 business_cache），
// 也不引入 Viper。下游：领域 provider/do 用 domain/scenario 常量调用 Resolve
// 取得最终 Policy，再交给领域 cache 装配 Store 与 TTL。
//
// 边界与维护：
//   - 目录构造后不可变（内部对 definition 做深拷贝），Resolve 返回值类型，
//     不暴露内部 map；禁止在此叠加运行时热更新、动态注册或默认 TTL 回退。
//   - 场景缺失/启用场景 TTL 非法一律 fail-fast，由启动失败暴露配置漂移。
//   - 新增策略维度（如 per-domain Redis）前必须先有第二个真实用例。
package cache

import (
	"errors"
	"fmt"
	"time"
)

// ErrPolicyDomainNotFound 策略目录中不存在指定 domain 声明。
var ErrPolicyDomainNotFound = errors.New("cache policy domain not declared")

// ErrPolicyScenarioNotFound 策略目录中指定 domain 下不存在指定 scenario 声明。
var ErrPolicyScenarioNotFound = errors.New("cache policy scenario not declared")

// PolicyDefinition 业务缓存策略定义的顶层通用结构。
// 由 app 配置 provider 从 YAML 根 section 反序列化；字段名即配置 key（snake case 由 mapstructure tag 决定）。
type PolicyDefinition struct {
	// Enabled 全局总开关；false 时所有场景解析结果均禁用。
	Enabled bool `mapstructure:"enabled"`
	// RedisInstance 业务缓存统一使用的 Redis 实例名（只允许全局指定，不支持 domain/scenario 覆盖）。
	RedisInstance string `mapstructure:"redis_instance"`
	// Domains 按业务域分组的策略声明；key 为领域配置 ID。
	Domains map[string]DomainPolicyDefinition `mapstructure:"domains"`
}

// DomainPolicyDefinition 单个业务域的策略声明。
type DomainPolicyDefinition struct {
	// Enabled 领域开关；false 时该域全部场景禁用。
	Enabled bool `mapstructure:"enabled"`
	// Scenarios 按场景分组的策略声明；key 为场景配置 ID。
	Scenarios map[string]ScenarioPolicyDefinition `mapstructure:"scenarios"`
}

// ScenarioPolicyDefinition 单个缓存场景的策略声明。
type ScenarioPolicyDefinition struct {
	// Enabled 场景开关；与全局、领域开关共同决定最终是否启用。
	Enabled bool `mapstructure:"enabled"`
	// TTL 缓存有效期；启用场景必须大于 0，禁用场景允许为 0 以便再开启时补齐。
	TTL time.Duration `mapstructure:"ttl"`
}

// Policy Resolve 输出的最终生效策略（值类型，调用方可安全持有）。
type Policy struct {
	// Enabled 有效开关 = 全局 && 领域 && 场景 三级开关合取。
	Enabled bool
	// TTL 场景声明的缓存有效期；仅 Enabled 时被领域 cache 使用。
	TTL time.Duration
	// RedisInstance 全局声明的 Redis 实例名。
	RedisInstance string
}

// PolicyCatalog 不可变的业务缓存策略目录。
type PolicyCatalog struct {
	redisInstance string
	globalEnabled bool
	domains       map[string]DomainPolicyDefinition
}

// NewPolicyCatalog 校验并冻结策略定义；定义非法时返回错误，不做默认值回退。
// 校验规则：全局启用时必须声明 RedisInstance；domain/scenario 名不得为空；
// 启用场景的 TTL 必须大于 0（禁用场景允许 TTL 为 0）。
func NewPolicyCatalog(def PolicyDefinition) (*PolicyCatalog, error) {
	if def.Enabled && def.RedisInstance == "" {
		return nil, fmt.Errorf("cache policy: redis_instance is required when enabled")
	}
	domains := make(map[string]DomainPolicyDefinition, len(def.Domains))
	for name, domainDef := range def.Domains {
		if name == "" {
			return nil, fmt.Errorf("cache policy: domain name is required")
		}
		scenarios := make(map[string]ScenarioPolicyDefinition, len(domainDef.Scenarios))
		for scenarioName, scenarioDef := range domainDef.Scenarios {
			if scenarioName == "" {
				return nil, fmt.Errorf("cache policy: domain %s scenario name is required", name)
			}
			if scenarioDef.Enabled && scenarioDef.TTL <= 0 {
				return nil, fmt.Errorf("cache policy: domain %s scenario %s: ttl must be positive when enabled", name, scenarioName)
			}
			scenarios[scenarioName] = scenarioDef
		}
		domains[name] = DomainPolicyDefinition{Enabled: domainDef.Enabled, Scenarios: scenarios}
	}
	return &PolicyCatalog{redisInstance: def.RedisInstance, globalEnabled: def.Enabled, domains: domains}, nil
}

// Resolve 返回指定 domain/scenario 的最终策略；声明缺失返回错误，
// 由调用方（领域 provider）在启动时 fail-fast，不静默回退默认 TTL。
func (c *PolicyCatalog) Resolve(domain, scenario string) (Policy, error) {
	if c == nil {
		return Policy{}, fmt.Errorf("cache policy: catalog is nil")
	}
	domainDef, ok := c.domains[domain]
	if !ok {
		return Policy{}, fmt.Errorf("%w: %s", ErrPolicyDomainNotFound, domain)
	}
	scenarioDef, ok := domainDef.Scenarios[scenario]
	if !ok {
		return Policy{}, fmt.Errorf("%w: %s/%s", ErrPolicyScenarioNotFound, domain, scenario)
	}
	return Policy{
		Enabled:       c.globalEnabled && domainDef.Enabled && scenarioDef.Enabled,
		TTL:           scenarioDef.TTL,
		RedisInstance: c.redisInstance,
	}, nil
}
