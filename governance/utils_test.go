package governance

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetLocalIP(t *testing.T) {
	ip, err := GetLocalIP()
	// May or may not return error depending on environment
	// But should always return a valid IP format
	assert.NotEmpty(t, ip)
	// Should be a valid IP format
	if err == nil {
		assert.NotEqual(t, "127.0.0.1", ip) // Should be non-loopback if successful
	}
}

func TestFormatServiceAddress(t *testing.T) {
	tests := []struct {
		host     string
		port     int
		expected string
	}{
		{"127.0.0.1", 8080, "127.0.0.1:8080"},
		{"192.168.1.100", 9002, "192.168.1.100:9002"},
		{"localhost", 80, "localhost:80"},
		{"::1", 443, "[::1]:443"}, // IPv6
	}

	for _, tt := range tests {
		result := FormatServiceAddress(tt.host, tt.port)
		assert.Equal(t, tt.expected, result)
	}
}

func TestParseServiceAddress(t *testing.T) {
	t.Run("valid address", func(t *testing.T) {
		host, port, err := ParseServiceAddress("127.0.0.1:8080")
		assert.NoError(t, err)
		assert.Equal(t, "127.0.0.1", host)
		assert.Equal(t, 8080, port)
	})

	t.Run("IPv6 address", func(t *testing.T) {
		host, port, err := ParseServiceAddress("[::1]:443")
		assert.NoError(t, err)
		assert.Equal(t, "::1", host)
		assert.Equal(t, 443, port)
	})

	t.Run("invalid format - no port", func(t *testing.T) {
		_, _, err := ParseServiceAddress("127.0.0.1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid address format")
	})

	t.Run("invalid port", func(t *testing.T) {
		_, _, err := ParseServiceAddress("127.0.0.1:abc")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid port")
	})
}

func TestGenerateInstanceID(t *testing.T) {
	tests := []struct {
		serviceName string
		address     string
		port        int
		expected    string
	}{
		{"auth-service", "192.168.1.100", 9002, "auth-service-192.168.1.100-9002"},
		{"api-gateway", "10.0.0.1", 8080, "api-gateway-10.0.0.1-8080"},
	}

	for _, tt := range tests {
		result := GenerateInstanceID(tt.serviceName, tt.address, tt.port)
		assert.Equal(t, tt.expected, result)
	}
}

func TestServiceInfo_Validate_EdgeCases(t *testing.T) {
	t.Run("port at upper limit", func(t *testing.T) {
		info := &ServiceInfo{
			ServiceName: "test-service",
			Address:     "127.0.0.1",
			Port:        65535,
		}
		err := info.Validate()
		assert.NoError(t, err)
	})

	t.Run("port over limit", func(t *testing.T) {
		info := &ServiceInfo{
			ServiceName: "test-service",
			Address:     "127.0.0.1",
			Port:        65536,
		}
		err := info.Validate()
		assert.Error(t, err)
		assert.Equal(t, ErrInvalidPort, err)
	})

	t.Run("custom TTL preserved", func(t *testing.T) {
		info := &ServiceInfo{
			ServiceName: "test-service",
			Address:     "127.0.0.1",
			Port:        8080,
			TTL:         30,
		}
		err := info.Validate()
		assert.NoError(t, err)
		assert.Equal(t, int64(30), info.TTL)
	})

	t.Run("custom protocol preserved", func(t *testing.T) {
		info := &ServiceInfo{
			ServiceName: "test-service",
			Address:     "127.0.0.1",
			Port:        8080,
			Protocol:    "http",
		}
		err := info.Validate()
		assert.NoError(t, err)
		assert.Equal(t, "http", info.Protocol)
	})

	t.Run("custom instanceID preserved", func(t *testing.T) {
		info := &ServiceInfo{
			ServiceName: "test-service",
			Address:     "127.0.0.1",
			Port:        8080,
			InstanceID:  "my-custom-id",
		}
		err := info.Validate()
		assert.NoError(t, err)
		assert.Equal(t, "my-custom-id", info.InstanceID)
	})
}
