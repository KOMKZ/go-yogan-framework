package health

import (
	"context"
	"errors"
	"testing"
	"time"
)

// mockChecker 模拟健康检查器
type mockChecker struct {
	name string
	err  error
}

func (m *mockChecker) Name() string {
	return m.name
}

func (m *mockChecker) Check(ctx context.Context) error {
	return m.err
}

func TestAggregator_Check(t *testing.T) {
	tests := []struct {
		name     string
		checkers []Checker
		want     Status
	}{
		{
			name:     "无检查项",
			checkers: []Checker{},
			want:     StatusHealthy,
		},
		{
			name: "所有检查项健康",
			checkers: []Checker{
				&mockChecker{name: "db", err: nil},
				&mockChecker{name: "redis", err: nil},
			},
			want: StatusHealthy,
		},
		{
			name: "部分检查项不健康",
			checkers: []Checker{
				&mockChecker{name: "db", err: nil},
				&mockChecker{name: "redis", err: errors.New("connection failed")},
			},
			want: StatusUnhealthy,
		},
		{
			name: "所有检查项不健康",
			checkers: []Checker{
				&mockChecker{name: "db", err: errors.New("db down")},
				&mockChecker{name: "redis", err: errors.New("redis down")},
			},
			want: StatusUnhealthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agg := NewAggregator(time.Second)
			for _, checker := range tt.checkers {
				agg.Register(checker)
			}

			response := agg.Check(context.Background())

			if response.Status != tt.want {
				t.Errorf("Aggregator.Check() status = %v, want %v", response.Status, tt.want)
			}

			if len(response.Checks) != len(tt.checkers) {
				t.Errorf("Aggregator.Check() checks count = %d, want %d", len(response.Checks), len(tt.checkers))
			}
		})
	}
}

func TestAggregator_SetMetadata(t *testing.T) {
	agg := NewAggregator(time.Second)
	agg.SetMetadata("service", "test-service")
	agg.SetMetadata("version", "1.0.0")

	response := agg.Check(context.Background())

	if response.Metadata["service"] != "test-service" {
		t.Errorf("Expected service metadata to be test-service")
	}
	if response.Metadata["version"] != "1.0.0" {
		t.Errorf("Expected version metadata to be 1.0.0")
	}
}

func TestResponse_IsHealthy(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   bool
	}{
		{"healthy", StatusHealthy, true},
		{"degraded", StatusDegraded, false},
		{"unhealthy", StatusUnhealthy, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := &Response{Status: tt.status}
			if got := response.IsHealthy(); got != tt.want {
				t.Errorf("Response.IsHealthy() = %v, want %v", got, tt.want)
			}
		})
	}
}

