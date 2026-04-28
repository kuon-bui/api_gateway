package middleware

import (
	"net/http"
	"sync"
	"time"

	"api-gateway/internal/app"
	"api-gateway/internal/config"
	"api-gateway/internal/domain"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type clientLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type limiterStore struct {
	mu      sync.Mutex
	clients map[string]*clientLimiter
	rps     rate.Limit
	burst   int
}

func newLimiterStore(rps, burst int) *limiterStore {
	return &limiterStore{
		clients: make(map[string]*clientLimiter),
		rps:     rate.Limit(rps),
		burst:   burst,
	}
}

func (s *limiterStore) get(key string) *rate.Limiter {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.clients[key]
	if !ok {
		entry = &clientLimiter{limiter: rate.NewLimiter(s.rps, s.burst)}
		s.clients[key] = entry
	}
	entry.lastSeen = time.Now()
	return entry.limiter
}

func (s *limiterStore) gc(ttl time.Duration) {
	ticker := time.NewTicker(ttl)
	defer ticker.Stop()
	for range ticker.C {
		cutoff := time.Now().Add(-ttl)
		s.mu.Lock()
		for key, client := range s.clients {
			if client.lastSeen.Before(cutoff) {
				delete(s.clients, key)
			}
		}
		s.mu.Unlock()
	}
}

func RateLimit(globalCfg config.RateLimitConfig, resolver *app.Resolver) gin.HandlerFunc {
	headerName := globalCfg.APIKeyHeader
	if headerName == "" {
		headerName = "X-API-Key"
	}

	var globalStore *limiterStore
	if globalCfg.Enabled {
		globalStore = newLimiterStore(globalCfg.RPS, globalCfg.Burst)
		go globalStore.gc(10 * time.Minute)
	}

	routeStores := make(map[string]*limiterStore)
	var routeStoresMu sync.Mutex
	getRouteStore := func(route domain.Route) *limiterStore {
		if route.RateLimit == nil || !route.RateLimit.Enabled {
			return nil
		}

		routeStoresMu.Lock()
		defer routeStoresMu.Unlock()

		store, ok := routeStores[route.Name]
		if ok {
			return store
		}

		store = newLimiterStore(route.RateLimit.RPS, route.RateLimit.Burst)
		routeStores[route.Name] = store
		go store.gc(10 * time.Minute)
		return store
	}

	if globalStore == nil && resolver == nil {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	return func(c *gin.Context) {
		key := c.GetHeader(headerName)
		if key == "" {
			key = c.ClientIP()
		}

		var limiter *rate.Limiter
		if resolver != nil {
			route, ok := resolver.Match(c.Request.Method, c.Request.URL.Path)
			if ok && route.RateLimit != nil {
				if !route.RateLimit.Enabled {
					c.Next()
					return
				}
				if store := getRouteStore(route); store != nil {
					limiter = store.get(key)
				}
			}
		}

		if limiter == nil {
			if globalStore == nil {
				c.Next()
				return
			}
			limiter = globalStore.get(key)
		}

		if !limiter.Allow() {
			domain.WriteError(c, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", "Too many requests")
			c.Abort()
			return
		}
		c.Next()
	}
}
