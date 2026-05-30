package middleware

import (
	"net/http/httptest"
	"testing"
	"time"

	"api-gateway/internal/config"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
)

func TestRedisRateLimiterAllowsBurstThenBlocks(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mini, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mini.Close()

	backend := newRedisRateLimiterBackend(config.RateLimitConfig{
		RedisAddress:   mini.Addr(),
		RedisKeyPrefix: "test:rl",
	})

	ctx := redisTestContext()
	scope := "route:users"
	key := "client-a"

	allowed, err := backend.Allow(ctx, scope, key, 1, 2)
	if err != nil {
		t.Fatalf("Allow returned error: %v", err)
	}
	if !allowed {
		t.Fatal("expected first request to be allowed")
	}

	allowed, err = backend.Allow(ctx, scope, key, 1, 2)
	if err != nil {
		t.Fatalf("Allow returned error: %v", err)
	}
	if !allowed {
		t.Fatal("expected second request to be allowed within burst")
	}

	allowed, err = backend.Allow(ctx, scope, key, 1, 2)
	if err != nil {
		t.Fatalf("Allow returned error: %v", err)
	}
	if allowed {
		t.Fatal("expected third request to be blocked after burst")
	}
}

func TestRedisRateLimiterRefillsTokens(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mini, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mini.Close()

	backend := newRedisRateLimiterBackend(config.RateLimitConfig{
		RedisAddress:   mini.Addr(),
		RedisKeyPrefix: "test:rl",
	})

	ctx := redisTestContext()
	scope := "global"
	key := "client-b"

	allowed, err := backend.Allow(ctx, scope, key, 5, 1)
	if err != nil {
		t.Fatalf("Allow returned error: %v", err)
	}
	if !allowed {
		t.Fatal("expected first request to be allowed")
	}

	allowed, err = backend.Allow(ctx, scope, key, 5, 1)
	if err != nil {
		t.Fatalf("Allow returned error: %v", err)
	}
	if allowed {
		t.Fatal("expected immediate second request to be blocked")
	}

	time.Sleep(250 * time.Millisecond)

	allowed, err = backend.Allow(ctx, scope, key, 5, 1)
	if err != nil {
		t.Fatalf("Allow returned error: %v", err)
	}
	if !allowed {
		t.Fatal("expected request to be allowed after token refill")
	}
}

func redisTestContext() *gin.Context {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("GET", "/", nil)
	return ctx
}
