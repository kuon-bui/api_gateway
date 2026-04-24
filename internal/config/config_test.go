package config

import "testing"

func TestValidateSuccess(t *testing.T) {
	cfg := Config{
		Server:    ServerConfig{Port: 8080, ReadTimeoutMS: 1, WriteTimeoutMS: 1, IdleTimeoutMS: 1},
		Proxy:     ProxyConfig{TimeoutMS: 1},
		Security:  SecurityConfig{JWT: JWTConfig{Enabled: true, HMACSecret: "secret"}},
		RateLimit: RateLimitConfig{Enabled: true, RPS: 1, Burst: 1, APIKeyHeader: "X-API-Key"},
		Routes:    []RouteConfig{{Name: "users", Methods: []string{"GET"}, PathPrefix: "/users", Upstream: "http://localhost:9001"}},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid config, got error: %v", err)
	}
}

func TestValidateFailsWithoutRoutes(t *testing.T) {
	cfg := Config{
		Server: ServerConfig{Port: 8080, ReadTimeoutMS: 1, WriteTimeoutMS: 1, IdleTimeoutMS: 1},
		Proxy:  ProxyConfig{TimeoutMS: 1},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}
