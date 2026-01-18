package event

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRouter_Match_ExactMatch(t *testing.T) {
	router := NewRouter()
	router.LoadRoutes(map[string]RouteConfig{
		"order.created": {Driver: DriverKafka, Topic: "events.order"},
		"user.login":    {Driver: DriverKafka, Topic: "events.user"},
	})

	// 精确匹配
	route := router.Match("order.created")
	assert.NotNil(t, route)
	assert.Equal(t, DriverKafka, route.Driver)
	assert.Equal(t, "events.order", route.Topic)

	// 不匹配
	route = router.Match("order.updated")
	assert.Nil(t, route)
}

func TestRouter_Match_WildcardSuffix(t *testing.T) {
	router := NewRouter()
	router.LoadRoutes(map[string]RouteConfig{
		"order.*": {Driver: DriverKafka, Topic: "events.order"},
	})

	// 通配符匹配
	assert.NotNil(t, router.Match("order.created"))
	assert.NotNil(t, router.Match("order.updated"))
	assert.NotNil(t, router.Match("order.cancelled"))

	// 不匹配
	assert.Nil(t, router.Match("user.login"))
	assert.Nil(t, router.Match("order")) // 没有点
}

func TestRouter_Match_UniversalWildcard(t *testing.T) {
	router := NewRouter()
	router.LoadRoutes(map[string]RouteConfig{
		"*": {Driver: DriverKafka, Topic: "events.all"},
	})

	// 匹配所有
	assert.NotNil(t, router.Match("order.created"))
	assert.NotNil(t, router.Match("user.login"))
	assert.NotNil(t, router.Match("anything"))
}

func TestRouter_Match_Priority(t *testing.T) {
	router := NewRouter()
	router.LoadRoutes(map[string]RouteConfig{
		"*":             {Driver: DriverKafka, Topic: "events.all"},
		"order.*":       {Driver: DriverKafka, Topic: "events.order"},
		"order.created": {Driver: DriverKafka, Topic: "events.order.created"},
	})

	// 精确匹配优先
	route := router.Match("order.created")
	assert.NotNil(t, route)
	assert.Equal(t, "events.order.created", route.Topic)

	// 通配符次之
	route = router.Match("order.updated")
	assert.NotNil(t, route)
	assert.Equal(t, "events.order", route.Topic)

	// 通用通配符最后
	route = router.Match("user.login")
	assert.NotNil(t, route)
	assert.Equal(t, "events.all", route.Topic)
}

func TestRouter_Match_MiddleWildcard(t *testing.T) {
	router := NewRouter()
	router.LoadRoutes(map[string]RouteConfig{
		"order.*.done": {Driver: DriverKafka, Topic: "events.order.done"},
	})

	// 中间通配符
	assert.NotNil(t, router.Match("order.created.done"))
	assert.NotNil(t, router.Match("order.updated.done"))

	// 不匹配
	assert.Nil(t, router.Match("order.created"))
	assert.Nil(t, router.Match("order.done"))
	assert.Nil(t, router.Match("order.created.updated.done")) // 层级不对
}

func TestRouter_HasRoutes(t *testing.T) {
	router := NewRouter()
	assert.False(t, router.HasRoutes())

	router.LoadRoutes(map[string]RouteConfig{
		"order.*": {Driver: DriverKafka, Topic: "events.order"},
	})
	assert.True(t, router.HasRoutes())
	assert.Equal(t, 1, router.RouteCount())
}

func TestRouter_EmptyRoutes(t *testing.T) {
	router := NewRouter()

	// 空路由返回 nil
	assert.Nil(t, router.Match("order.created"))
}
