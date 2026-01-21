package governance

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractInstanceIDFromKey(t *testing.T) {
	tests := []struct {
		key      string
		expected string
	}{
		{"/services/test-service/instance-1", "instance-1"},
		{"/services/test-service/192.168.1.1-8080", "192.168.1.1-8080"},
		{"instance-only", "instance-only"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result := extractInstanceIDFromKey(tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseAddress(t *testing.T) {
	tests := []struct {
		addr     string
		expected string
	}{
		{"127.0.0.1:9002", "127.0.0.1"},
		{"192.168.1.100:8080", "192.168.1.100"},
		{"localhost:80", "localhost"},
		{"no-port", "no-port"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			result := parseAddress(tt.addr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParsePort(t *testing.T) {
	tests := []struct {
		addr     string
		expected int
	}{
		{"127.0.0.1:9002", 9002},
		{"192.168.1.100:8080", 8080},
		{"localhost:443", 443},
		{"no-port", 0},
		{"", 0},
		{"invalid:", 0},
	}

	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			result := parsePort(tt.addr)
			assert.Equal(t, tt.expected, result)
		})
	}
}
