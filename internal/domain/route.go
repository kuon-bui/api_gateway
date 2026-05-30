package domain

import "net/url"

type Route struct {
	Name           string
	Methods        map[string]struct{}
	PathPrefix     string
	Upstream       *url.URL
	TrimPath       bool
	ForwardHeaders map[string]struct{}
	CircuitBreaker *RouteCircuitBreaker
	Retry          *RouteRetry
	RateLimit      *RouteRateLimit
}

type RouteRateLimit struct {
	Enabled bool
	RPS     int
	Burst   int
}

type RouteCircuitBreaker struct {
	Enabled             bool
	FailureThreshold    uint32
	OpenTimeout         int
	HalfOpenMaxRequests uint32
}

type RouteRetry struct {
	Enabled     bool
	MaxAttempts int
	BackoffMS   int
}
