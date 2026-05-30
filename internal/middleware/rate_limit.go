package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"api-gateway/internal/app"
	"api-gateway/internal/config"
	"api-gateway/internal/domain"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
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

type rateLimiterBackend interface {
	Allow(c *gin.Context, scope, key string, rps, burst int) (bool, error)
}

type memoryRateLimiterBackend struct {
	mu     sync.Mutex
	stores map[string]*limiterStore
}

func newMemoryRateLimiterBackend() *memoryRateLimiterBackend {
	return &memoryRateLimiterBackend{stores: make(map[string]*limiterStore)}
}

func (b *memoryRateLimiterBackend) Allow(_ *gin.Context, scope, key string, rps, burst int) (bool, error) {
	store := b.storeForScope(scope, rps, burst)
	return store.get(key).Allow(), nil
}

func (b *memoryRateLimiterBackend) storeForScope(scope string, rps, burst int) *limiterStore {
	b.mu.Lock()
	defer b.mu.Unlock()

	if store, ok := b.stores[scope]; ok {
		return store
	}

	store := newLimiterStore(rps, burst)
	b.stores[scope] = store
	go store.gc(10 * time.Minute)
	return store
}

type redisRateLimiterBackend struct {
	client    *redis.Client
	keyPrefix string
	now       func() time.Time
}

var redisTokenBucketScript = redis.NewScript(`
local key = KEYS[1]
local now_ms = tonumber(ARGV[1])
local refill_rate = tonumber(ARGV[2])
local burst = tonumber(ARGV[3])
local requested = tonumber(ARGV[4])

if refill_rate <= 0 or burst <= 0 or requested <= 0 then
	return {0, 0}
end

local data = redis.call("HMGET", key, "tokens", "ts")
local tokens = tonumber(data[1])
local ts = tonumber(data[2])

if tokens == nil then
	tokens = burst
end
if ts == nil then
	ts = now_ms
end

local delta_ms = now_ms - ts
if delta_ms < 0 then
	delta_ms = 0
end

tokens = math.min(burst, tokens + (delta_ms / 1000.0) * refill_rate)

local allowed = 0
if tokens >= requested then
	allowed = 1
	tokens = tokens - requested
end

redis.call("HMSET", key, "tokens", tokens, "ts", now_ms)

local ttl_ms = math.ceil((burst / refill_rate) * 1000) + 1000
if ttl_ms < 1000 then
	ttl_ms = 1000
end
redis.call("PEXPIRE", key, ttl_ms)

return {allowed, tokens}
`)

func newRedisRateLimiterBackend(cfg config.RateLimitConfig) *redisRateLimiterBackend {
	prefix := strings.TrimSpace(cfg.RedisKeyPrefix)
	if prefix == "" {
		prefix = "gateway:rl"
	}

	client := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddress,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	return &redisRateLimiterBackend{
		client:    client,
		keyPrefix: prefix,
		now:       time.Now,
	}
}

func (b *redisRateLimiterBackend) Allow(c *gin.Context, scope, key string, rps, burst int) (bool, error) {
	if rps <= 0 || burst <= 0 {
		return true, nil
	}

	ctx := c.Request.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	redisKey := fmt.Sprintf("%s:%s:%s", b.keyPrefix, scope, key)
	nowMs := b.now().UnixMilli()

	result, err := redisTokenBucketScript.Run(
		ctx,
		b.client,
		[]string{redisKey},
		nowMs,
		rps,
		burst,
		1,
	).Result()
	if err != nil {
		return false, err
	}

	allowed, err := parseAllowedResult(result)
	if err != nil {
		return false, err
	}

	return allowed, nil
}

func parseAllowedResult(result any) (bool, error) {
	values, ok := result.([]any)
	if !ok || len(values) == 0 {
		return false, fmt.Errorf("unexpected redis limiter result: %T", result)
	}

	allowedRaw, ok := values[0].(int64)
	if !ok {
		return false, fmt.Errorf("unexpected redis limiter allowed type: %T", values[0])
	}

	return allowedRaw == 1, nil
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

	if !globalCfg.Enabled && resolver == nil {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	backendName := strings.ToLower(strings.TrimSpace(globalCfg.Backend))
	if backendName == "" {
		backendName = "memory"
	}

	var backend rateLimiterBackend
	if backendName == "redis" {
		backend = newRedisRateLimiterBackend(globalCfg)
	} else {
		backend = newMemoryRateLimiterBackend()
	}

	return func(c *gin.Context) {
		key := c.GetHeader(headerName)
		if key == "" {
			key = c.ClientIP()
		}

		enabled := globalCfg.Enabled
		rps := globalCfg.RPS
		burst := globalCfg.Burst
		scope := "global"

		if resolver != nil {
			route, ok := resolver.Match(c.Request.Method, c.Request.URL.Path)
			if ok && route.RateLimit != nil {
				if !route.RateLimit.Enabled {
					c.Next()
					return
				}
				enabled = true
				rps = route.RateLimit.RPS
				burst = route.RateLimit.Burst
				scope = "route:" + route.Name
			}
		}

		if !enabled {
			c.Next()
			return
		}

		allowed, err := backend.Allow(c, scope, key, rps, burst)
		if err != nil {
			domain.WriteError(c, http.StatusServiceUnavailable, "RATE_LIMIT_BACKEND_ERROR", "Rate limit backend unavailable")
			c.Abort()
			return
		}
		if !allowed {
			domain.WriteError(c, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", "Too many requests")
			c.Abort()
			return
		}
		c.Next()
	}
}
