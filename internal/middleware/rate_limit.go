package middleware

import (
	"net/http"
	"sync"
	"time"

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

func RateLimit(cfg config.RateLimitConfig) gin.HandlerFunc {
	if !cfg.Enabled {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	store := newLimiterStore(cfg.RPS, cfg.Burst)
	go store.gc(10 * time.Minute)

	headerName := cfg.APIKeyHeader
	if headerName == "" {
		headerName = "X-API-Key"
	}

	return func(c *gin.Context) {
		key := c.GetHeader(headerName)
		if key == "" {
			key = c.ClientIP()
		}
		if !store.get(key).Allow() {
			domain.WriteError(c, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", "Too many requests")
			c.Abort()
			return
		}
		c.Next()
	}
}
