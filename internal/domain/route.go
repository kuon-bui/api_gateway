package domain

import "net/url"

type Route struct {
	Name       string
	Methods    map[string]struct{}
	PathPrefix string
	Upstream   *url.URL
	TrimPath   bool
	RateLimit  *RouteRateLimit
}

type RouteRateLimit struct {
	Enabled bool
	RPS     int
	Burst   int
}
