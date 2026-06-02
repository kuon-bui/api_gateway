package app

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"api-gateway/internal/balancer"
	"api-gateway/internal/config"
	"api-gateway/internal/domain"
)

type Resolver struct {
	mu     sync.RWMutex
	routes []domain.Route
}

func NewResolver(cfg config.Config) (*Resolver, error) {
	routes, err := buildRoutes(cfg)
	if err != nil {
		return nil, err
	}

	return &Resolver{routes: routes}, nil
}

func buildRoutes(cfg config.Config) ([]domain.Route, error) {
	routes := make([]domain.Route, 0, len(cfg.Routes))
	for _, rt := range cfg.Routes {
		bal, err := buildBalancer(rt)
		if err != nil {
			return nil, fmt.Errorf("build balancer for route %q: %w", rt.Name, err)
		}

		methods := make(map[string]struct{}, len(rt.Methods))
		for _, m := range rt.Methods {
			methods[strings.ToUpper(m)] = struct{}{}
		}

		forwardHeaders := make(map[string]struct{}, len(rt.ForwardHeaders))
		for _, header := range rt.ForwardHeaders {
			if header == "" {
				continue
			}
			forwardHeaders[http.CanonicalHeaderKey(strings.TrimSpace(header))] = struct{}{}
		}

		routes = append(routes, domain.Route{
			Name:           rt.Name,
			Methods:        methods,
			PathPrefix:     rt.PathPrefix,
			Balancer:       bal,
			TrimPath:       rt.TrimPath,
			BypassCORS:     rt.BypassCORS,
			ForwardHeaders: forwardHeaders,
			CircuitBreaker: toDomainCircuitBreaker(rt.CircuitBreaker),
			Retry:          toDomainRetry(rt.Retry),
			RateLimit:      toDomainRouteRateLimit(rt.RateLimit),
		})
	}

	sort.Slice(routes, func(i, j int) bool {
		return len(routes[i].PathPrefix) > len(routes[j].PathPrefix)
	})

	return routes, nil
}

func (r *Resolver) Reload(cfg config.Config) error {
	routes, err := buildRoutes(cfg)
	if err != nil {
		return err
	}

	r.mu.Lock()
	r.routes = routes
	r.mu.Unlock()
	return nil
}

func (r *Resolver) Snapshot() []domain.Route {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]domain.Route, len(r.routes))
	copy(out, r.routes)
	return out
}

// buildBalancer constructs a load balancer for a route from either the single
// `upstream` field or the `upstreams` pool. A single upstream yields a
// one-element pool, preserving backward compatibility.
func buildBalancer(rt config.RouteConfig) (*balancer.Balancer, error) {
	var upstreams []*balancer.Upstream

	if len(rt.Upstreams) > 0 {
		for _, u := range rt.Upstreams {
			parsed, err := url.Parse(strings.TrimSpace(u.URL))
			if err != nil {
				return nil, fmt.Errorf("parse upstream url %q: %w", u.URL, err)
			}
			weight := u.Weight
			if weight <= 0 {
				weight = 1
			}
			upstreams = append(upstreams, &balancer.Upstream{URL: parsed, Weight: weight})
		}
	} else {
		parsed, err := url.Parse(strings.TrimSpace(rt.Upstream))
		if err != nil {
			return nil, fmt.Errorf("parse upstream url %q: %w", rt.Upstream, err)
		}
		upstreams = append(upstreams, &balancer.Upstream{URL: parsed, Weight: 1})
	}

	strategy := balancer.RoundRobin
	switch strings.ToLower(strings.TrimSpace(rt.LoadBalancing)) {
	case string(balancer.Weighted):
		strategy = balancer.Weighted
	case string(balancer.Random):
		strategy = balancer.Random
	}

	var passive balancer.PassiveHealth
	if rt.HealthCheck != nil {
		passive = balancer.PassiveHealth{
			Enabled:          rt.HealthCheck.Passive.Enabled,
			FailureThreshold: rt.HealthCheck.Passive.FailureThreshold,
			Cooldown:         time.Duration(rt.HealthCheck.Passive.CooldownMS) * time.Millisecond,
		}
	}

	return balancer.New(upstreams, strategy, passive), nil
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

func toDomainCircuitBreaker(cfg *config.RouteCircuitBreakerConfig) *domain.RouteCircuitBreaker {
	if cfg == nil {
		return nil
	}
	return &domain.RouteCircuitBreaker{
		Enabled:             cfg.Enabled,
		FailureThreshold:    cfg.FailureThreshold,
		OpenTimeout:         cfg.OpenTimeoutMS,
		HalfOpenMaxRequests: cfg.HalfOpenMaxRequests,
	}
}

func toDomainRetry(cfg *config.RouteRetryConfig) *domain.RouteRetry {
	if cfg == nil {
		return nil
	}
	return &domain.RouteRetry{
		Enabled:     cfg.Enabled,
		MaxAttempts: cfg.MaxAttempts,
		BackoffMS:   cfg.BackoffMS,
	}
}

func (r *Resolver) Match(method, path string) (domain.Route, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

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
