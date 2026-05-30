package middleware

import (
	"testing"
)

func TestResolveMetricRouteLabel(t *testing.T) {
	tests := []struct {
		name        string
		routeName   any
		fullPath    string
		requestPath string
		wantLabel   string
	}{
		{
			name:        "uses route name when proxy route is set",
			routeName:   "users-service",
			requestPath: "/users/42",
			wantLabel:   "users-service",
		},
		{
			name:        "uses full path for matched gin route",
			requestPath: "/healthz",
			fullPath:    "/healthz",
			wantLabel:   "/healthz",
		},
		{
			name:        "system route fallback",
			requestPath: "/metrics",
			wantLabel:   "/metrics",
		},
		{
			name:        "unmatched fallback",
			requestPath: "/dynamic/123",
			wantLabel:   "unmatched",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveMetricRouteLabel(tt.routeName, tt.fullPath, tt.requestPath); got != tt.wantLabel {
				t.Fatalf("resolveMetricRouteLabel() = %q, want %q", got, tt.wantLabel)
			}
		})
	}
}
