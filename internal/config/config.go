package config

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const insecureJWTSecretPlaceholder = "change-me-in-prod"

// Config holds all gateway runtime configuration loaded from YAML/env.
type Config struct {
	Server    ServerConfig    `yaml:"server"    mapstructure:"server"`
	Proxy     ProxyConfig     `yaml:"proxy"     mapstructure:"proxy"`
	Security  SecurityConfig  `yaml:"security"  mapstructure:"security"`
	Admin     AdminConfig     `yaml:"admin"     mapstructure:"admin"`
	Telemetry TelemetryConfig `yaml:"telemetry" mapstructure:"telemetry"`
	RateLimit RateLimitConfig `yaml:"rate_limit" mapstructure:"rate_limit"`
	Routes    []RouteConfig   `yaml:"routes"    mapstructure:"routes"`
}

type ServerConfig struct {
	Port           int             `yaml:"port"            mapstructure:"port"`
	ReadTimeoutMS  int             `yaml:"read_timeout_ms" mapstructure:"read_timeout_ms"`
	WriteTimeoutMS int             `yaml:"write_timeout_ms" mapstructure:"write_timeout_ms"`
	IdleTimeoutMS  int             `yaml:"idle_timeout_ms" mapstructure:"idle_timeout_ms"`
	TLS            TLSServerConfig `yaml:"tls" mapstructure:"tls"`
}

type TLSServerConfig struct {
	Enabled  bool   `yaml:"enabled" mapstructure:"enabled"`
	CertFile string `yaml:"cert_file" mapstructure:"cert_file"`
	KeyFile  string `yaml:"key_file" mapstructure:"key_file"`
}

type ProxyConfig struct {
	TimeoutMS int `yaml:"timeout_ms" mapstructure:"timeout_ms"`
}

type SecurityConfig struct {
	JWT JWTConfig `yaml:"jwt" mapstructure:"jwt"`
}

type AdminConfig struct {
	Enabled bool   `yaml:"enabled" mapstructure:"enabled"`
	APIKey  string `yaml:"api_key" mapstructure:"api_key"`
}

type TelemetryConfig struct {
	Enabled      bool   `yaml:"enabled" mapstructure:"enabled"`
	ServiceName  string `yaml:"service_name" mapstructure:"service_name"`
	OTLPEndpoint string `yaml:"otlp_endpoint" mapstructure:"otlp_endpoint"`
	Insecure     bool   `yaml:"insecure" mapstructure:"insecure"`
}

type JWTConfig struct {
	Enabled    bool   `yaml:"enabled"     mapstructure:"enabled"`
	Issuer     string `yaml:"issuer"      mapstructure:"issuer"`
	Audience   string `yaml:"audience"    mapstructure:"audience"`
	HMACSecret string `yaml:"hmac_secret" mapstructure:"hmac_secret"`
}

type RateLimitConfig struct {
	Enabled        bool     `yaml:"enabled"          mapstructure:"enabled"`
	RPS            int      `yaml:"rps"              mapstructure:"rps"`
	Burst          int      `yaml:"burst"            mapstructure:"burst"`
	APIKeyHeader   string   `yaml:"by_api_key_header" mapstructure:"by_api_key_header"`
	Backend        string   `yaml:"backend" mapstructure:"backend"`
	RedisAddress   string   `yaml:"redis_address" mapstructure:"redis_address"`
	RedisPassword  string   `yaml:"redis_password" mapstructure:"redis_password"`
	RedisDB        int      `yaml:"redis_db" mapstructure:"redis_db"`
	RedisKeyPrefix string   `yaml:"redis_key_prefix" mapstructure:"redis_key_prefix"`
	TrustedProxies []string `yaml:"trusted_proxies"  mapstructure:"trusted_proxies"`
}

type RouteRateLimitConfig struct {
	Enabled bool `yaml:"enabled" mapstructure:"enabled"`
	RPS     int  `yaml:"rps"     mapstructure:"rps"`
	Burst   int  `yaml:"burst"   mapstructure:"burst"`
}

type RouteCircuitBreakerConfig struct {
	Enabled             bool   `yaml:"enabled" mapstructure:"enabled"`
	FailureThreshold    uint32 `yaml:"failure_threshold" mapstructure:"failure_threshold"`
	OpenTimeoutMS       int    `yaml:"open_timeout_ms" mapstructure:"open_timeout_ms"`
	HalfOpenMaxRequests uint32 `yaml:"half_open_max_requests" mapstructure:"half_open_max_requests"`
}

type RouteRetryConfig struct {
	Enabled     bool `yaml:"enabled" mapstructure:"enabled"`
	MaxAttempts int  `yaml:"max_attempts" mapstructure:"max_attempts"`
	BackoffMS   int  `yaml:"backoff_ms" mapstructure:"backoff_ms"`
}

type RouteConfig struct {
	Name           string                     `yaml:"name"        mapstructure:"name"`
	Methods        []string                   `yaml:"methods"     mapstructure:"methods"`
	PathPrefix     string                     `yaml:"path_prefix" mapstructure:"path_prefix"`
	Upstream       string                     `yaml:"upstream"    mapstructure:"upstream"`
	TrimPath       bool                       `yaml:"trim_path"   mapstructure:"trim_path"`
	ForwardHeaders []string                   `yaml:"forward_headers,omitempty" mapstructure:"forward_headers"`
	CircuitBreaker *RouteCircuitBreakerConfig `yaml:"circuit_breaker,omitempty" mapstructure:"circuit_breaker"`
	Retry          *RouteRetryConfig          `yaml:"retry,omitempty" mapstructure:"retry"`
	RateLimit      *RouteRateLimitConfig      `yaml:"rate_limit,omitempty" mapstructure:"rate_limit"`
}

func (c Config) Validate() error {
	if c.Server.Port <= 0 {
		return errors.New("server.port must be positive")
	}
	if c.Server.ReadTimeoutMS <= 0 || c.Server.IdleTimeoutMS <= 0 {
		return errors.New("server read_timeout_ms and idle_timeout_ms must be positive")
	}
	if c.Server.WriteTimeoutMS < 0 {
		return errors.New("server.write_timeout_ms must be zero or positive")
	}
	if c.Server.TLS.Enabled {
		if strings.TrimSpace(c.Server.TLS.CertFile) == "" || strings.TrimSpace(c.Server.TLS.KeyFile) == "" {
			return errors.New("server.tls.cert_file and server.tls.key_file are required when TLS is enabled")
		}
	}
	if c.Proxy.TimeoutMS < 0 {
		return errors.New("proxy.timeout_ms must be zero or positive")
	}
	if c.Security.JWT.Enabled && c.Security.JWT.HMACSecret == "" {
		return errors.New("security.jwt.hmac_secret is required when JWT is enabled")
	}
	if c.Security.JWT.Enabled {
		if c.Security.JWT.HMACSecret == insecureJWTSecretPlaceholder {
			return errors.New("security.jwt.hmac_secret must not use insecure placeholder value")
		}
		if len(c.Security.JWT.HMACSecret) < 32 {
			return errors.New("security.jwt.hmac_secret must be at least 32 characters")
		}
	}
	if c.RateLimit.Enabled {
		if c.RateLimit.RPS <= 0 || c.RateLimit.Burst <= 0 {
			return errors.New("rate_limit.rps and rate_limit.burst must be positive")
		}
		if c.RateLimit.APIKeyHeader == "" {
			c.RateLimit.APIKeyHeader = "X-API-Key"
		}
		if c.RateLimit.Backend == "" {
			c.RateLimit.Backend = "memory"
		}
		backend := strings.ToLower(strings.TrimSpace(c.RateLimit.Backend))
		if backend != "memory" && backend != "redis" {
			return errors.New("rate_limit.backend must be either 'memory' or 'redis'")
		}
		if backend == "redis" && strings.TrimSpace(c.RateLimit.RedisAddress) == "" {
			return errors.New("rate_limit.redis_address is required when rate_limit.backend is redis")
		}
	}
	if c.Admin.Enabled && strings.TrimSpace(c.Admin.APIKey) == "" {
		return errors.New("admin.api_key is required when admin is enabled")
	}
	if c.Telemetry.Enabled {
		if strings.TrimSpace(c.Telemetry.OTLPEndpoint) == "" {
			return errors.New("telemetry.otlp_endpoint is required when telemetry is enabled")
		}
		if strings.TrimSpace(c.Telemetry.ServiceName) == "" {
			c.Telemetry.ServiceName = "api-gateway"
		}
	}
	if len(c.Routes) == 0 {
		return errors.New("routes must not be empty")
	}

	seenNames := make(map[string]struct{}, len(c.Routes))
	for i, rt := range c.Routes {
		if rt.Name == "" {
			return fmt.Errorf("routes[%d].name must not be empty", i)
		}
		if _, ok := seenNames[rt.Name]; ok {
			return fmt.Errorf("routes[%d].name duplicated: %s", i, rt.Name)
		}
		seenNames[rt.Name] = struct{}{}

		if !strings.HasPrefix(rt.PathPrefix, "/") {
			return fmt.Errorf("routes[%d].path_prefix must start with '/'", i)
		}
		if hasSystemRouteConflict(rt.PathPrefix) {
			return fmt.Errorf("routes[%d].path_prefix conflicts with reserved system endpoints", i)
		}
		if len(rt.Methods) == 0 {
			return fmt.Errorf("routes[%d].methods must not be empty", i)
		}
		for _, method := range rt.Methods {
			if method == "" {
				return fmt.Errorf("routes[%d].methods contains empty value", i)
			}
			if !isHTTPMethod(method) {
				return fmt.Errorf("routes[%d].methods contains invalid method: %s", i, method)
			}
		}
		if _, err := url.ParseRequestURI(rt.Upstream); err != nil {
			return fmt.Errorf("routes[%d].upstream is invalid: %w", i, err)
		}
		for _, header := range rt.ForwardHeaders {
			if strings.TrimSpace(header) == "" {
				return fmt.Errorf("routes[%d].forward_headers contains empty value", i)
			}
		}
		if rt.RateLimit != nil && rt.RateLimit.Enabled {
			if rt.RateLimit.RPS <= 0 || rt.RateLimit.Burst <= 0 {
				return fmt.Errorf("routes[%d].rate_limit.rps and routes[%d].rate_limit.burst must be positive", i, i)
			}
		}
		if rt.CircuitBreaker != nil && rt.CircuitBreaker.Enabled {
			if rt.CircuitBreaker.FailureThreshold == 0 {
				return fmt.Errorf("routes[%d].circuit_breaker.failure_threshold must be positive", i)
			}
			if rt.CircuitBreaker.OpenTimeoutMS <= 0 {
				return fmt.Errorf("routes[%d].circuit_breaker.open_timeout_ms must be positive", i)
			}
			if rt.CircuitBreaker.HalfOpenMaxRequests == 0 {
				return fmt.Errorf("routes[%d].circuit_breaker.half_open_max_requests must be positive", i)
			}
		}
		if rt.Retry != nil && rt.Retry.Enabled {
			if rt.Retry.MaxAttempts <= 1 {
				return fmt.Errorf("routes[%d].retry.max_attempts must be greater than 1", i)
			}
			if rt.Retry.BackoffMS < 0 {
				return fmt.Errorf("routes[%d].retry.backoff_ms must be zero or positive", i)
			}
		}
	}

	return nil
}

func (c Config) ReadTimeout() time.Duration {
	return time.Duration(c.Server.ReadTimeoutMS) * time.Millisecond
}

func (c Config) WriteTimeout() time.Duration {
	return time.Duration(c.Server.WriteTimeoutMS) * time.Millisecond
}

func (c Config) IdleTimeout() time.Duration {
	return time.Duration(c.Server.IdleTimeoutMS) * time.Millisecond
}

func (c Config) ProxyTimeout() time.Duration {
	return time.Duration(c.Proxy.TimeoutMS) * time.Millisecond
}

func isHTTPMethod(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

func hasSystemRouteConflict(routePrefix string) bool {
	reservedEndpoints := []string{"/healthz", "/readyz", "/metrics"}
	for _, endpoint := range reservedEndpoints {
		if strings.HasPrefix(endpoint, routePrefix) || strings.HasPrefix(routePrefix, endpoint) {
			return true
		}
	}
	return false
}
