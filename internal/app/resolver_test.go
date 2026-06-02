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

func TestResolverBuildsUpstreamPool(t *testing.T) {
	cfg := config.Config{
		Routes: []config.RouteConfig{
			{
				Name:          "pool",
				Methods:       []string{"GET"},
				PathPrefix:    "/pool",
				LoadBalancing: "weighted",
				Upstreams: []config.UpstreamConfig{
					{URL: "http://a:9001", Weight: 3},
					{URL: "http://b:9002", Weight: 1},
				},
			},
			{Name: "single", Methods: []string{"GET"}, PathPrefix: "/single", Upstream: "http://c:9003"},
		},
	}

	resolver, err := NewResolver(cfg)
	if err != nil {
		t.Fatalf("NewResolver returned error: %v", err)
	}

	pool, ok := resolver.Match("GET", "/pool/x")
	if !ok {
		t.Fatal("expected pool route match")
	}
	ups := pool.Balancer.Upstreams()
	if len(ups) != 2 {
		t.Fatalf("expected 2 upstreams, got %d", len(ups))
	}
	if ups[0].Weight != 3 || ups[1].Weight != 1 {
		t.Fatalf("unexpected weights: %d, %d", ups[0].Weight, ups[1].Weight)
	}

	single, ok := resolver.Match("GET", "/single/x")
	if !ok {
		t.Fatal("expected single route match")
	}
	singleUps := single.Balancer.Upstreams()
	if len(singleUps) != 1 || singleUps[0].URL.String() != "http://c:9003" {
		t.Fatalf("expected single-element pool from upstream field, got %+v", singleUps)
	}
	if singleUps[0].Weight != 1 {
		t.Fatalf("expected default weight 1 for single upstream, got %d", singleUps[0].Weight)
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
				TrimPath:   "/api",
				RateLimit: &config.RouteRateLimitConfig{
					Enabled: true,
					RPS:     10,
					Burst:   20,
				},
			},
			{Name: "passthrough", Methods: []string{"GET"}, PathPrefix: "/assets", Upstream: "http://localhost:9002", TrimPath: ""},
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
	if trimmedRoute.TrimPath != "/api" {
		t.Fatal("expected trim_path=\"/api\" to be preserved")
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
	if passthroughRoute.TrimPath != "" {
		t.Fatal("expected trim_path=\"\" to be preserved")
	}
	if passthroughRoute.RateLimit != nil {
		t.Fatal("expected nil route rate_limit when not configured")
	}
}
