package app

import (
	"testing"

	"api-gateway/internal/config"
)

func TestResolverMatchLongestPrefix(t *testing.T) {
	cfg := config.Config{
		Routes: []config.RouteConfig{
			{Name: "default-users", Methods: []string{"GET"}, PathPrefix: "/users", Upstream: "http://localhost:9001"},
			{Name: "users-detail", Methods: []string{"GET"}, PathPrefix: "/users/detail", Upstream: "http://localhost:9002"},
		},
	}

	resolver, err := NewResolver(cfg)
	if err != nil {
		t.Fatalf("NewResolver returned error: %v", err)
	}

	route, ok := resolver.Match("GET", "/users/detail/123")
	if !ok {
		t.Fatal("expected route match")
	}
	if route.Name != "users-detail" {
		t.Fatalf("expected users-detail route, got %s", route.Name)
	}
}

func TestResolverCarriesTrimPath(t *testing.T) {
	cfg := config.Config{
		Routes: []config.RouteConfig{
			{
				Name:       "trimmed",
				Methods:    []string{"GET"},
				PathPrefix: "/api",
				Upstream:   "http://localhost:9001",
				TrimPath:   true,
				RateLimit: &config.RouteRateLimitConfig{
					Enabled: true,
					RPS:     10,
					Burst:   20,
				},
			},
			{Name: "passthrough", Methods: []string{"GET"}, PathPrefix: "/assets", Upstream: "http://localhost:9002", TrimPath: false},
		},
	}

	resolver, err := NewResolver(cfg)
	if err != nil {
		t.Fatalf("NewResolver returned error: %v", err)
	}

	trimmedRoute, ok := resolver.Match("GET", "/api/users")
	if !ok {
		t.Fatal("expected trimmed route match")
	}
	if !trimmedRoute.TrimPath {
		t.Fatal("expected trim_path=true to be preserved")
	}
	if trimmedRoute.RateLimit == nil {
		t.Fatal("expected route rate_limit config to be preserved")
	}
	if !trimmedRoute.RateLimit.Enabled || trimmedRoute.RateLimit.RPS != 10 || trimmedRoute.RateLimit.Burst != 20 {
		t.Fatalf("unexpected route rate_limit values: %+v", *trimmedRoute.RateLimit)
	}

	passthroughRoute, ok := resolver.Match("GET", "/assets/logo.svg")
	if !ok {
		t.Fatal("expected passthrough route match")
	}
	if passthroughRoute.TrimPath {
		t.Fatal("expected trim_path=false to be preserved")
	}
	if passthroughRoute.RateLimit != nil {
		t.Fatal("expected nil route rate_limit when not configured")
	}
}
