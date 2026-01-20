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

	// Exact match
	route := router.Match("order.created")
	assert.NotNil(t, route)
	assert.Equal(t, DriverKafka, route.Driver)
	assert.Equal(t, "events.order", route.Topic)

	// mismatched
	route = router.Match("order.updated")
	assert.Nil(t, route)
}

func TestRouter_Match_WildcardSuffix(t *testing.T) {
	router := NewRouter()
	router.LoadRoutes(map[string]RouteConfig{
		"order.*": {Driver: DriverKafka, Topic: "events.order"},
	})

	// wildcard matching
	assert.NotNil(t, router.Match("order.created"))
	assert.NotNil(t, router.Match("order.updated"))
	assert.NotNil(t, router.Match("order.cancelled"))

	// mismatch
	assert.Nil(t, router.Match("user.login"))
	assert.Nil(t, router.Match("order")) // There is no dot
}

func TestRouter_Match_UniversalWildcard(t *testing.T) {
	router := NewRouter()
	router.LoadRoutes(map[string]RouteConfig{
		"*": {Driver: DriverKafka, Topic: "events.all"},
	})

	// Match all
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

	// Prefer exact matches
	route := router.Match("order.created")
	assert.NotNil(t, route)
	assert.Equal(t, "events.order.created", route.Topic)

	// Wildcard next priority
	route = router.Match("order.updated")
	assert.NotNil(t, route)
	assert.Equal(t, "events.order", route.Topic)

	// General wildcard at last
	route = router.Match("user.login")
	assert.NotNil(t, route)
	assert.Equal(t, "events.all", route.Topic)
}

func TestRouter_Match_MiddleWildcard(t *testing.T) {
	router := NewRouter()
	router.LoadRoutes(map[string]RouteConfig{
		"order.*.done": {Driver: DriverKafka, Topic: "events.order.done"},
	})

	// middle wildcard
	assert.NotNil(t, router.Match("order.created.done"))
	assert.NotNil(t, router.Match("order.updated.done"))

	// mismatch
	assert.Nil(t, router.Match("order.created"))
	assert.Nil(t, router.Match("order.done"))
	assert.Nil(t, router.Match("order.created.updated.done")) // Hierarchy mismatch
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

	// An empty route returns nil
	assert.Nil(t, router.Match("order.created"))
}
