package config

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Config holds all gateway runtime configuration loaded from YAML/env.
type Config struct {
	Server    ServerConfig    `yaml:"server"    mapstructure:"server"`
	Proxy     ProxyConfig     `yaml:"proxy"     mapstructure:"proxy"`
	Security  SecurityConfig  `yaml:"security"  mapstructure:"security"`
	RateLimit RateLimitConfig `yaml:"rate_limit" mapstructure:"rate_limit"`
	Routes    []RouteConfig   `yaml:"routes"    mapstructure:"routes"`
}

type ServerConfig struct {
	Port           int `yaml:"port"            mapstructure:"port"`
	ReadTimeoutMS  int `yaml:"read_timeout_ms" mapstructure:"read_timeout_ms"`
	WriteTimeoutMS int `yaml:"write_timeout_ms" mapstructure:"write_timeout_ms"`
	IdleTimeoutMS  int `yaml:"idle_timeout_ms" mapstructure:"idle_timeout_ms"`
}

type ProxyConfig struct {
	TimeoutMS int `yaml:"timeout_ms" mapstructure:"timeout_ms"`
}

type SecurityConfig struct {
	JWT JWTConfig `yaml:"jwt" mapstructure:"jwt"`
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
	TrustedProxies []string `yaml:"trusted_proxies"  mapstructure:"trusted_proxies"`
}

type RouteConfig struct {
	Name       string   `yaml:"name"        mapstructure:"name"`
	Methods    []string `yaml:"methods"     mapstructure:"methods"`
	PathPrefix string   `yaml:"path_prefix" mapstructure:"path_prefix"`
	Upstream   string   `yaml:"upstream"    mapstructure:"upstream"`
}

func (c Config) Validate() error {
	if c.Server.Port <= 0 {
		return errors.New("server.port must be positive")
	}
	if c.Server.ReadTimeoutMS <= 0 || c.Server.WriteTimeoutMS <= 0 || c.Server.IdleTimeoutMS <= 0 {
		return errors.New("server timeouts must be positive")
	}
	if c.Proxy.TimeoutMS <= 0 {
		return errors.New("proxy.timeout_ms must be positive")
	}
	if c.Security.JWT.Enabled && c.Security.JWT.HMACSecret == "" {
		return errors.New("security.jwt.hmac_secret is required when JWT is enabled")
	}
	if c.RateLimit.Enabled {
		if c.RateLimit.RPS <= 0 || c.RateLimit.Burst <= 0 {
			return errors.New("rate_limit.rps and rate_limit.burst must be positive")
		}
		if c.RateLimit.APIKeyHeader == "" {
			c.RateLimit.APIKeyHeader = "X-API-Key"
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
