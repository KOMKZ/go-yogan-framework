// 本文件验证 PolicyCatalog 的构造校验与解析语义：三级开关合取、TTL 透传、
// 声明缺失 fail-fast、非法定义拒绝构造、目录构造后不可变。
// 维护时新增策略规则必须同步补齐对应用例，保持校验规则与测试一一对应。
package cache

import (
	"errors"
	"testing"
	"time"
)

// validPolicyDefinition 返回一份最小合法定义，测试按需局部覆盖。
func validPolicyDefinition() PolicyDefinition {
	return PolicyDefinition{
		Enabled:       true,
		RedisInstance: "main",
		Domains: map[string]DomainPolicyDefinition{
			"domain-a": {
				Enabled: true,
				Scenarios: map[string]ScenarioPolicyDefinition{
					"scenario-on":  {Enabled: true, TTL: 45 * time.Second},
					"scenario-off": {Enabled: false},
				},
			},
			"domain-disabled": {
				Enabled: false,
				Scenarios: map[string]ScenarioPolicyDefinition{
					"scenario-on": {Enabled: true, TTL: 30 * time.Second},
				},
			},
		},
	}
}

func TestNewPolicyCatalogRejectsInvalidDefinitions(t *testing.T) {
	cases := []struct {
		name string
		def  PolicyDefinition
	}{
		{
			name: "global enabled without redis instance",
			def:  PolicyDefinition{Enabled: true, Domains: map[string]DomainPolicyDefinition{}},
		},
		{
			name: "empty domain name",
			def: PolicyDefinition{
				Enabled:       true,
				RedisInstance: "main",
				Domains:       map[string]DomainPolicyDefinition{"": {Enabled: true, Scenarios: map[string]ScenarioPolicyDefinition{"s": {Enabled: true, TTL: time.Second}}}},
			},
		},
		{
			name: "empty scenario name",
			def: PolicyDefinition{
				Enabled:       true,
				RedisInstance: "main",
				Domains: map[string]DomainPolicyDefinition{
					"d": {Enabled: true, Scenarios: map[string]ScenarioPolicyDefinition{"": {Enabled: true, TTL: time.Second}}},
				},
			},
		},
		{
			name: "enabled scenario with zero ttl",
			def: PolicyDefinition{
				Enabled:       true,
				RedisInstance: "main",
				Domains: map[string]DomainPolicyDefinition{
					"d": {Enabled: true, Scenarios: map[string]ScenarioPolicyDefinition{"s": {Enabled: true}}},
				},
			},
		},
		{
			name: "enabled scenario with negative ttl",
			def: PolicyDefinition{
				Enabled:       true,
				RedisInstance: "main",
				Domains: map[string]DomainPolicyDefinition{
					"d": {Enabled: true, Scenarios: map[string]ScenarioPolicyDefinition{"s": {Enabled: true, TTL: -time.Second}}},
				},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewPolicyCatalog(tc.def); err == nil {
				t.Fatalf("expected construction error")
			}
		})
	}
}

func TestNewPolicyCatalogAllowsDisabledScenarioWithoutTTL(t *testing.T) {
	def := validPolicyDefinition()
	if _, err := NewPolicyCatalog(def); err != nil {
		t.Fatalf("construct catalog: %v", err)
	}
}

func TestResolveCombinesThreeLevelSwitches(t *testing.T) {
	def := validPolicyDefinition()
	// 全局关闭时所有场景解析为禁用。
	def.Enabled = false
	catalog, err := NewPolicyCatalog(def)
	if err != nil {
		t.Fatalf("construct catalog: %v", err)
	}
	policy, err := catalog.Resolve("domain-a", "scenario-on")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if policy.Enabled {
		t.Fatalf("policy should be disabled when global switch is off")
	}

	catalog, err = NewPolicyCatalog(validPolicyDefinition())
	if err != nil {
		t.Fatalf("construct catalog: %v", err)
	}
	// 全开：启用。
	policy, err = catalog.Resolve("domain-a", "scenario-on")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !policy.Enabled || policy.TTL != 45*time.Second || policy.RedisInstance != "main" {
		t.Fatalf("policy = %#v", policy)
	}
	// 场景关：禁用，TTL 仍透传。
	policy, err = catalog.Resolve("domain-a", "scenario-off")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if policy.Enabled || policy.TTL != 0 {
		t.Fatalf("policy = %#v", policy)
	}
	// 领域关：禁用。
	policy, err = catalog.Resolve("domain-disabled", "scenario-on")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if policy.Enabled || policy.TTL != 30*time.Second {
		t.Fatalf("policy = %#v", policy)
	}
}

func TestResolveFailsFastOnMissingDeclarations(t *testing.T) {
	catalog, err := NewPolicyCatalog(validPolicyDefinition())
	if err != nil {
		t.Fatalf("construct catalog: %v", err)
	}
	if _, err := catalog.Resolve("domain-missing", "scenario-on"); !errors.Is(err, ErrPolicyDomainNotFound) {
		t.Fatalf("err = %v, want ErrPolicyDomainNotFound", err)
	}
	if _, err := catalog.Resolve("domain-a", "scenario-missing"); !errors.Is(err, ErrPolicyScenarioNotFound) {
		t.Fatalf("err = %v, want ErrPolicyScenarioNotFound", err)
	}
}

func TestPolicyCatalogIsImmutableAfterConstruction(t *testing.T) {
	def := validPolicyDefinition()
	catalog, err := NewPolicyCatalog(def)
	if err != nil {
		t.Fatalf("construct catalog: %v", err)
	}
	// 构造后篡改原 definition（含嵌套 map）不得影响目录。
	def.Domains["domain-a"].Scenarios["scenario-on"] = ScenarioPolicyDefinition{Enabled: true, TTL: time.Hour}
	delete(def.Domains, "domain-disabled")

	policy, err := catalog.Resolve("domain-a", "scenario-on")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if policy.TTL != 45*time.Second {
		t.Fatalf("policy.TTL = %v, want 45s (mutation leaked)", policy.TTL)
	}
	if _, err := catalog.Resolve("domain-disabled", "scenario-on"); err != nil {
		t.Fatalf("resolve domain-disabled after caller mutation: %v", err)
	}
	// Resolve 返回值类型，两次解析互相独立。
	first, _ := catalog.Resolve("domain-a", "scenario-on")
	first.TTL = time.Minute
	second, _ := catalog.Resolve("domain-a", "scenario-on")
	if second.TTL != 45*time.Second {
		t.Fatalf("second resolve polluted by caller: %v", second.TTL)
	}
}
