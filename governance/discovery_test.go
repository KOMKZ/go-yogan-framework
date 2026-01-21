package governance

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServiceInstance_GetAddress(t *testing.T) {
	instance := &ServiceInstance{
		Address: "192.168.1.100",
		Port:    9002,
	}

	assert.Equal(t, "192.168.1.100:9002", instance.GetAddress())
}

func TestServiceInstance_GetAddress_IPv6(t *testing.T) {
	instance := &ServiceInstance{
		Address: "::1",
		Port:    443,
	}

	assert.Equal(t, "[::1]:443", instance.GetAddress())
}
