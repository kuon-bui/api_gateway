package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"api-gateway/internal/app"
	"api-gateway/internal/config"

	"github.com/gin-gonic/gin"
)

func TestRateLimitRouteOverrideEnabledWhenGlobalDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := newRateLimitTestEngine(t, config.RateLimitConfig{Enabled: false}, []config.RouteConfig{
		{
			Name:       "users",
			Methods:    []string{"GET"},
			PathPrefix: "/users",
			Upstream:   "http://localhost:9001",
			RateLimit: &config.RouteRateLimitConfig{
				Enabled: true,
				RPS:     1,
				Burst:   1,
			},
		},
	})

	gateway := httptest.NewServer(engine)
	defer gateway.Close()

	first := doRequest(t, gateway.URL, "/users/me", "client-a")
	defer first.Body.Close()
	if first.StatusCode != http.StatusNoContent {
		t.Fatalf("expected first request status 204, got %d", first.StatusCode)
	}

	second := doRequest(t, gateway.URL, "/users/me", "client-a")
	defer second.Body.Close()
	if second.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected second request status 429, got %d", second.StatusCode)
	}
}

func TestRateLimitRouteOverrideDisabledSkipsGlobalLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := newRateLimitTestEngine(t, config.RateLimitConfig{Enabled: true, RPS: 1, Burst: 1}, []config.RouteConfig{
		{
			Name:       "users",
			Methods:    []string{"GET"},
			PathPrefix: "/users",
			Upstream:   "http://localhost:9001",
			RateLimit: &config.RouteRateLimitConfig{
				Enabled: false,
			},
		},
	})

	gateway := httptest.NewServer(engine)
	defer gateway.Close()

	first := doRequest(t, gateway.URL, "/users/me", "client-a")
	defer first.Body.Close()
	if first.StatusCode != http.StatusNoContent {
		t.Fatalf("expected first request status 204, got %d", first.StatusCode)
	}

	second := doRequest(t, gateway.URL, "/users/me", "client-a")
	defer second.Body.Close()
	if second.StatusCode != http.StatusNoContent {
		t.Fatalf("expected second request status 204, got %d", second.StatusCode)
	}
}

func TestRateLimitUsesGlobalLimitWhenRouteHasNoOverride(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := newRateLimitTestEngine(t, config.RateLimitConfig{Enabled: true, RPS: 1, Burst: 1}, []config.RouteConfig{
		{
			Name:       "users",
			Methods:    []string{"GET"},
			PathPrefix: "/users",
			Upstream:   "http://localhost:9001",
		},
	})

	gateway := httptest.NewServer(engine)
	defer gateway.Close()

	first := doRequest(t, gateway.URL, "/users/me", "client-a")
	defer first.Body.Close()
	if first.StatusCode != http.StatusNoContent {
		t.Fatalf("expected first request status 204, got %d", first.StatusCode)
	}

	second := doRequest(t, gateway.URL, "/users/me", "client-a")
	defer second.Body.Close()
	if second.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected second request status 429, got %d", second.StatusCode)
	}
}

func newRateLimitTestEngine(t *testing.T, globalCfg config.RateLimitConfig, routes []config.RouteConfig) *gin.Engine {
	t.Helper()

	resolver, err := app.NewResolver(config.Config{Routes: routes})
	if err != nil {
		t.Fatalf("failed to build resolver: %v", err)
	}

	engine := gin.New()
	engine.Use(RateLimit(globalCfg, resolver))
	engine.NoRoute(func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	return engine
}

func doRequest(t *testing.T, baseURL, path, clientKey string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, baseURL+path, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Key", clientKey)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	return res
}
