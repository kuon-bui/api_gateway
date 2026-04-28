package app

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"api-gateway/internal/config"
	"api-gateway/internal/domain"

	"github.com/samber/lo"
)

type Resolver struct {
	routes []domain.Route
}

func NewResolver(cfg config.Config) (*Resolver, error) {
	routes := make([]domain.Route, 0, len(cfg.Routes))
	for _, rt := range cfg.Routes {
		upstreamURL, err := url.Parse(rt.Upstream)
		if err != nil {
			return nil, fmt.Errorf("parse upstream url for route %q: %w", rt.Name, err)
		}

		methods := make(map[string]struct{}, len(rt.Methods))
		for _, m := range rt.Methods {
			methods[strings.ToUpper(m)] = struct{}{}
		}
		routes = append(routes, domain.Route{
			Name:       rt.Name,
			Methods:    methods,
			PathPrefix: rt.PathPrefix,
			Upstream:   upstreamURL,
			TrimPath:   rt.TrimPath,
			RateLimit:  toDomainRouteRateLimit(rt.RateLimit),
		})
	}

	sort.Slice(routes, func(i, j int) bool {
		return len(routes[i].PathPrefix) > len(routes[j].PathPrefix)
	})
	fmt.Printf("Loaded %d routes:\n", len(routes))
	for _, rt := range routes {
		fmt.Printf("- %s %s -> %s\n", lo.Keys(rt.Methods), rt.PathPrefix, rt.Upstream)
	}

	return &Resolver{routes: routes}, nil
}

func toDomainRouteRateLimit(cfg *config.RouteRateLimitConfig) *domain.RouteRateLimit {
	if cfg == nil {
		return nil
	}
	return &domain.RouteRateLimit{
		Enabled: cfg.Enabled,
		RPS:     cfg.RPS,
		Burst:   cfg.Burst,
	}
}

func (r *Resolver) Match(method, path string) (domain.Route, bool) {
	method = strings.ToUpper(method)
	for _, route := range r.routes {
		if _, ok := route.Methods[method]; !ok {
			continue
		}
		if strings.HasPrefix(path, route.PathPrefix) {
			return route, true
		}
	}
	return domain.Route{}, false
}
