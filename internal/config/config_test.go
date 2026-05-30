package config

import "testing"

func TestValidateSuccess(t *testing.T) {
	cfg := Config{
		Server:    ServerConfig{Port: 8080, ReadTimeoutMS: 1, WriteTimeoutMS: 1, IdleTimeoutMS: 1},
		Proxy:     ProxyConfig{TimeoutMS: 1},
		Security:  SecurityConfig{JWT: JWTConfig{Enabled: true, HMACSecret: "0123456789abcdef0123456789abcdef"}},
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

func TestValidateAllowsZeroWriteTimeout(t *testing.T) {
	cfg := Config{
		Server: ServerConfig{Port: 8080, ReadTimeoutMS: 1, WriteTimeoutMS: 0, IdleTimeoutMS: 1},
		Proxy:  ProxyConfig{TimeoutMS: 1},
		Routes: []RouteConfig{{
			Name:       "events",
			Methods:    []string{"GET"},
			PathPrefix: "/events",
			Upstream:   "http://localhost:9001",
		}},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid config, got error: %v", err)
	}
}

func TestValidateFailsNegativeWriteTimeout(t *testing.T) {
	cfg := Config{
		Server: ServerConfig{Port: 8080, ReadTimeoutMS: 1, WriteTimeoutMS: -1, IdleTimeoutMS: 1},
		Proxy:  ProxyConfig{TimeoutMS: 1},
		Routes: []RouteConfig{{
			Name:       "events",
			Methods:    []string{"GET"},
			PathPrefix: "/events",
			Upstream:   "http://localhost:9001",
		}},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for negative write timeout")
	}
}

func TestValidateAllowsZeroProxyTimeout(t *testing.T) {
	cfg := Config{
		Server: ServerConfig{Port: 8080, ReadTimeoutMS: 1, WriteTimeoutMS: 0, IdleTimeoutMS: 1},
		Proxy:  ProxyConfig{TimeoutMS: 0},
		Routes: []RouteConfig{{
			Name:       "events",
			Methods:    []string{"GET"},
			PathPrefix: "/events",
			Upstream:   "http://localhost:9001",
		}},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid config, got error: %v", err)
	}
}

func TestValidateFailsNegativeProxyTimeout(t *testing.T) {
	cfg := Config{
		Server: ServerConfig{Port: 8080, ReadTimeoutMS: 1, WriteTimeoutMS: 0, IdleTimeoutMS: 1},
		Proxy:  ProxyConfig{TimeoutMS: -1},
		Routes: []RouteConfig{{
			Name:       "events",
			Methods:    []string{"GET"},
			PathPrefix: "/events",
			Upstream:   "http://localhost:9001",
		}},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for negative proxy timeout")
	}
}

func TestValidateAllowsEnabledRouteRateLimit(t *testing.T) {
	cfg := Config{
		Server: ServerConfig{Port: 8080, ReadTimeoutMS: 1, WriteTimeoutMS: 1, IdleTimeoutMS: 1},
		Proxy:  ProxyConfig{TimeoutMS: 1},
		Routes: []RouteConfig{{
			Name:       "events",
			Methods:    []string{"GET"},
			PathPrefix: "/events",
			Upstream:   "http://localhost:9001",
			RateLimit: &RouteRateLimitConfig{
				Enabled: true,
				RPS:     10,
				Burst:   20,
			},
		}},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid config, got error: %v", err)
	}
}

func TestValidateFailsRouteRateLimitWithInvalidValues(t *testing.T) {
	cfg := Config{
		Server: ServerConfig{Port: 8080, ReadTimeoutMS: 1, WriteTimeoutMS: 1, IdleTimeoutMS: 1},
		Proxy:  ProxyConfig{TimeoutMS: 1},
		Routes: []RouteConfig{{
			Name:       "events",
			Methods:    []string{"GET"},
			PathPrefix: "/events",
			Upstream:   "http://localhost:9001",
			RateLimit: &RouteRateLimitConfig{
				Enabled: true,
				RPS:     0,
				Burst:   20,
			},
		}},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for invalid route rate_limit values")
	}
}

func TestValidateFailsJWTPlaceholderSecret(t *testing.T) {
	cfg := Config{
		Server:   ServerConfig{Port: 8080, ReadTimeoutMS: 1, WriteTimeoutMS: 1, IdleTimeoutMS: 1},
		Proxy:    ProxyConfig{TimeoutMS: 1},
		Security: SecurityConfig{JWT: JWTConfig{Enabled: true, HMACSecret: "change-me-in-prod"}},
		Routes: []RouteConfig{{
			Name:       "events",
			Methods:    []string{"GET"},
			PathPrefix: "/events",
			Upstream:   "http://localhost:9001",
		}},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for insecure placeholder secret")
	}
}

func TestValidateFailsShortJWTSecret(t *testing.T) {
	cfg := Config{
		Server:   ServerConfig{Port: 8080, ReadTimeoutMS: 1, WriteTimeoutMS: 1, IdleTimeoutMS: 1},
		Proxy:    ProxyConfig{TimeoutMS: 1},
		Security: SecurityConfig{JWT: JWTConfig{Enabled: true, HMACSecret: "short"}},
		Routes: []RouteConfig{{
			Name:       "events",
			Methods:    []string{"GET"},
			PathPrefix: "/events",
			Upstream:   "http://localhost:9001",
		}},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for short secret")
	}
}

func TestValidateFailsReservedSystemRouteConflict(t *testing.T) {
	cfg := Config{
		Server: ServerConfig{Port: 8080, ReadTimeoutMS: 1, WriteTimeoutMS: 1, IdleTimeoutMS: 1},
		Proxy:  ProxyConfig{TimeoutMS: 1},
		Routes: []RouteConfig{{
			Name:       "conflict",
			Methods:    []string{"GET"},
			PathPrefix: "/",
			Upstream:   "http://localhost:9001",
		}},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for system route conflict")
	}
}

func TestValidateFailsTelemetryWithoutEndpoint(t *testing.T) {
	cfg := Config{
		Server:    ServerConfig{Port: 8080, ReadTimeoutMS: 1, WriteTimeoutMS: 1, IdleTimeoutMS: 1},
		Proxy:     ProxyConfig{TimeoutMS: 1},
		Telemetry: TelemetryConfig{Enabled: true, ServiceName: "api-gateway"},
		Routes: []RouteConfig{{
			Name:       "events",
			Methods:    []string{"GET"},
			PathPrefix: "/events",
			Upstream:   "http://localhost:9001",
		}},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error when telemetry endpoint is missing")
	}
}

func TestValidateAllowsTelemetryWithEndpoint(t *testing.T) {
	cfg := Config{
		Server:    ServerConfig{Port: 8080, ReadTimeoutMS: 1, WriteTimeoutMS: 1, IdleTimeoutMS: 1},
		Proxy:     ProxyConfig{TimeoutMS: 1},
		Telemetry: TelemetryConfig{Enabled: true, ServiceName: "api-gateway", OTLPEndpoint: "localhost:4317", Insecure: true},
		Routes: []RouteConfig{{
			Name:       "events",
			Methods:    []string{"GET"},
			PathPrefix: "/events",
			Upstream:   "http://localhost:9001",
		}},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid config, got error: %v", err)
	}
}

func TestValidateFailsTLSEnabledWithoutFiles(t *testing.T) {
	cfg := Config{
		Server: ServerConfig{
			Port:           8080,
			ReadTimeoutMS:  1,
			WriteTimeoutMS: 1,
			IdleTimeoutMS:  1,
			TLS:            TLSServerConfig{Enabled: true},
		},
		Proxy: ProxyConfig{TimeoutMS: 1},
		Routes: []RouteConfig{{
			Name:       "events",
			Methods:    []string{"GET"},
			PathPrefix: "/events",
			Upstream:   "http://localhost:9001",
		}},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for missing TLS cert/key files")
	}
}

func TestValidateFailsAdminEnabledWithoutAPIKey(t *testing.T) {
	cfg := Config{
		Server: ServerConfig{Port: 8080, ReadTimeoutMS: 1, WriteTimeoutMS: 1, IdleTimeoutMS: 1},
		Proxy:  ProxyConfig{TimeoutMS: 1},
		Admin:  AdminConfig{Enabled: true},
		Routes: []RouteConfig{{
			Name:       "events",
			Methods:    []string{"GET"},
			PathPrefix: "/events",
			Upstream:   "http://localhost:9001",
		}},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error when admin.api_key is missing")
	}
}

func baseRoute() RouteConfig {
	return RouteConfig{Name: "events", Methods: []string{"GET"}, PathPrefix: "/events"}
}

func cfgWith(rt RouteConfig) Config {
	return Config{
		Server: ServerConfig{Port: 8080, ReadTimeoutMS: 1, WriteTimeoutMS: 1, IdleTimeoutMS: 1},
		Proxy:  ProxyConfig{TimeoutMS: 1},
		Routes: []RouteConfig{rt},
	}
}

func TestValidateAllowsUpstreamPool(t *testing.T) {
	rt := baseRoute()
	rt.LoadBalancing = "weighted"
	rt.Upstreams = []UpstreamConfig{
		{URL: "http://a:9001", Weight: 3},
		{URL: "http://b:9001", Weight: 1},
	}
	if err := cfgWith(rt).Validate(); err != nil {
		t.Fatalf("expected valid upstream pool, got error: %v", err)
	}
}

func TestValidateFailsWhenNoUpstream(t *testing.T) {
	rt := baseRoute()
	if err := cfgWith(rt).Validate(); err == nil {
		t.Fatal("expected validation error when neither upstream nor upstreams is set")
	}
}

func TestValidateFailsWhenBothUpstreamForms(t *testing.T) {
	rt := baseRoute()
	rt.Upstream = "http://a:9001"
	rt.Upstreams = []UpstreamConfig{{URL: "http://b:9001", Weight: 1}}
	if err := cfgWith(rt).Validate(); err == nil {
		t.Fatal("expected validation error when both upstream and upstreams are set")
	}
}

func TestValidateFailsWeightedWithoutPositiveWeight(t *testing.T) {
	rt := baseRoute()
	rt.LoadBalancing = "weighted"
	rt.Upstreams = []UpstreamConfig{{URL: "http://a:9001", Weight: 0}}
	if err := cfgWith(rt).Validate(); err == nil {
		t.Fatal("expected validation error for non-positive weight under weighted strategy")
	}
}

func TestValidateFailsInvalidLoadBalancing(t *testing.T) {
	rt := baseRoute()
	rt.Upstream = "http://a:9001"
	rt.LoadBalancing = "least_conn"
	if err := cfgWith(rt).Validate(); err == nil {
		t.Fatal("expected validation error for unknown load_balancing strategy")
	}
}

func TestValidateFailsInvalidPoolUpstreamURL(t *testing.T) {
	rt := baseRoute()
	rt.Upstreams = []UpstreamConfig{{URL: "://broken", Weight: 1}}
	if err := cfgWith(rt).Validate(); err == nil {
		t.Fatal("expected validation error for invalid pool upstream url")
	}
}

func TestValidateFailsPassiveHealthMissingValues(t *testing.T) {
	rt := baseRoute()
	rt.Upstream = "http://a:9001"
	rt.HealthCheck = &RouteHealthCheckConfig{Passive: PassiveHealthConfig{Enabled: true, FailureThreshold: 0, CooldownMS: 1000}}
	if err := cfgWith(rt).Validate(); err == nil {
		t.Fatal("expected validation error when passive failure_threshold is zero")
	}
}

func TestValidateFailsRedisBackendWithoutAddress(t *testing.T) {
	cfg := Config{
		Server: ServerConfig{Port: 8080, ReadTimeoutMS: 1, WriteTimeoutMS: 1, IdleTimeoutMS: 1},
		Proxy:  ProxyConfig{TimeoutMS: 1},
		RateLimit: RateLimitConfig{
			Enabled:      true,
			RPS:          10,
			Burst:        20,
			APIKeyHeader: "X-API-Key",
			Backend:      "redis",
		},
		Routes: []RouteConfig{{
			Name:       "events",
			Methods:    []string{"GET"},
			PathPrefix: "/events",
			Upstream:   "http://localhost:9001",
		}},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error when redis backend has no address")
	}
}
