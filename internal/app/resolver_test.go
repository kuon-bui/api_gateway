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
