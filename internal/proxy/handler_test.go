package proxy

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"syscall"
	"testing"
	"time"

	"api-gateway/internal/app"
	"api-gateway/internal/config"

	"github.com/gin-gonic/gin"
)

func TestServeHTTPSupportsSSEStream(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("upstream response writer does not support flush")
		}

		_, _ = w.Write([]byte("data: first\n\n"))
		flusher.Flush()

		time.Sleep(120 * time.Millisecond)

		_, _ = w.Write([]byte("data: second\n\n"))
		flusher.Flush()
	}))
	defer upstream.Close()

	h := NewHandler(testResolver(t, upstream.URL), 40*time.Millisecond)
	engine := gin.New()
	engine.NoRoute(func(c *gin.Context) {
		h.ServeHTTP(c)
	})
	gateway := httptest.NewServer(engine)
	defer gateway.Close()

	req, err := http.NewRequest(http.MethodGet, gateway.URL+"/events/stream", nil)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	req.Header.Set("Accept", "text/event-stream")

	res, err := gateway.Client().Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = res.Body.Close() }()
	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	body := string(bodyBytes)

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d (body=%q)", res.StatusCode, body)
	}
	if !strings.Contains(body, "data: first") || !strings.Contains(body, "data: second") {
		t.Fatalf("expected full SSE stream in body, got %q", body)
	}
}

func TestServeHTTPNoLongerTimesOut(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(120 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	h := NewHandler(testResolver(t, upstream.URL), 40*time.Millisecond)
	engine := gin.New()
	engine.NoRoute(func(c *gin.Context) {
		h.ServeHTTP(c)
	})
	gateway := httptest.NewServer(engine)
	defer gateway.Close()

	res, err := gateway.Client().Get(gateway.URL + "/events/slow")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = res.Body.Close() }()
	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	body := string(bodyBytes)

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d (body=%q)", res.StatusCode, body)
	}
	if !strings.Contains(body, `{"ok":true}`) {
		t.Fatalf("expected successful upstream response, got %q", body)
	}
}

func TestServeHTTPTrimPathEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	pathCh := make(chan string, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pathCh <- r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	h := NewHandler(testResolverWithRoute(t, upstream.URL+"/api", "/events", true), 40*time.Millisecond)
	engine := gin.New()
	engine.NoRoute(func(c *gin.Context) {
		h.ServeHTTP(c)
	})
	gateway := httptest.NewServer(engine)
	defer gateway.Close()

	res, err := gateway.Client().Get(gateway.URL + "/events/stream")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", res.StatusCode)
	}

	select {
	case gotPath := <-pathCh:
		if gotPath != "/api/stream" {
			t.Fatalf("expected upstream path /api/stream, got %q", gotPath)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for upstream request")
	}
}

func TestServeHTTPTrimPathDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	pathCh := make(chan string, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pathCh <- r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	h := NewHandler(testResolverWithRoute(t, upstream.URL+"/api", "/events", false), 40*time.Millisecond)
	engine := gin.New()
	engine.NoRoute(func(c *gin.Context) {
		h.ServeHTTP(c)
	})
	gateway := httptest.NewServer(engine)
	defer gateway.Close()

	res, err := gateway.Client().Get(gateway.URL + "/events/stream")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", res.StatusCode)
	}

	select {
	case gotPath := <-pathCh:
		if gotPath != "/api/events/stream" {
			t.Fatalf("expected upstream path /api/events/stream, got %q", gotPath)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for upstream request")
	}
}

func TestServeHTTPFiltersForwardHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	headersCh := make(chan http.Header, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headersCh <- r.Header.Clone()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	resolver, err := app.NewResolver(config.Config{
		Routes: []config.RouteConfig{{
			Name:           "events",
			Methods:        []string{"GET"},
			PathPrefix:     "/events",
			Upstream:       upstream.URL,
			ForwardHeaders: []string{"X-Correlation-ID"},
		}},
	})
	if err != nil {
		t.Fatalf("failed to build resolver: %v", err)
	}

	h := NewHandler(resolver, 40*time.Millisecond)
	engine := gin.New()
	engine.NoRoute(func(c *gin.Context) {
		h.ServeHTTP(c)
	})
	gateway := httptest.NewServer(engine)
	defer gateway.Close()

	req, err := http.NewRequest(http.MethodGet, gateway.URL+"/events/filter", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("X-Correlation-ID", "corr-123")
	req.Header.Set("X-Other", "drop-me")
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Cookie", "session=abc")

	res, err := gateway.Client().Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", res.StatusCode)
	}

	select {
	case headers := <-headersCh:
		if got := headers.Get("X-Correlation-ID"); got != "corr-123" {
			t.Fatalf("expected X-Correlation-ID to be forwarded, got %q", got)
		}
		if got := headers.Get("X-Other"); got != "" {
			t.Fatalf("expected X-Other to be stripped, got %q", got)
		}
		if got := headers.Get("Authorization"); got != "" {
			t.Fatalf("expected Authorization to be stripped, got %q", got)
		}
		if got := headers.Get("Cookie"); got != "" {
			t.Fatalf("expected Cookie to be stripped, got %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for upstream request")
	}
}

func TestIsClientDisconnectPanic(t *testing.T) {
	tests := []struct {
		name string
		rec  any
		want bool
	}{
		{name: "http abort handler", rec: http.ErrAbortHandler, want: true},
		{name: "broken pipe text", rec: errors.New("write tcp 127.0.0.1: broken pipe"), want: true},
		{name: "connection reset text", rec: errors.New("read tcp 127.0.0.1: connection reset by peer"), want: true},
		{name: "syscall epipe", rec: syscall.EPIPE, want: true},
		{name: "regular error", rec: errors.New("some other error"), want: false},
		{name: "non error panic", rec: "panic string", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isClientDisconnectPanic(tt.rec); got != tt.want {
				t.Fatalf("isClientDisconnectPanic() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestServeHTTPFailsOverToHealthyUpstream(t *testing.T) {
	gin.SetMode(gin.TestMode)

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer bad.Close()

	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("from-good"))
	}))
	defer good.Close()

	resolver, err := app.NewResolver(config.Config{
		Routes: []config.RouteConfig{{
			Name:       "events",
			Methods:    []string{"GET"},
			PathPrefix: "/events",
			Upstreams: []config.UpstreamConfig{
				{URL: bad.URL, Weight: 1},
				{URL: good.URL, Weight: 1},
			},
			Retry: &config.RouteRetryConfig{Enabled: true, MaxAttempts: 2, BackoffMS: 0},
		}},
	})
	if err != nil {
		t.Fatalf("failed to build resolver: %v", err)
	}

	h := NewHandler(resolver, 200*time.Millisecond)
	engine := gin.New()
	engine.NoRoute(func(c *gin.Context) { h.ServeHTTP(c) })
	gateway := httptest.NewServer(engine)
	defer gateway.Close()

	// With two upstreams and two attempts, round-robin guarantees the healthy
	// upstream is reached within the retry budget regardless of ordering.
	res, err := gateway.Client().Get(gateway.URL + "/events/data")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = res.Body.Close() }()
	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 after failover, got %d", res.StatusCode)
	}
	if string(bodyBytes) != "from-good" {
		t.Fatalf("expected response from healthy upstream, got %q", string(bodyBytes))
	}
}

func TestServeHTTPReturnsNoHealthyUpstream(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Two upstreams pointing at refused ports; passive health ejects both after
	// the first request, so the second request short-circuits to 503.
	resolver, err := app.NewResolver(config.Config{
		Routes: []config.RouteConfig{{
			Name:       "events",
			Methods:    []string{"GET"},
			PathPrefix: "/events",
			Upstreams: []config.UpstreamConfig{
				{URL: "http://127.0.0.1:1", Weight: 1},
				{URL: "http://127.0.0.1:2", Weight: 1},
			},
			Retry:       &config.RouteRetryConfig{Enabled: true, MaxAttempts: 2, BackoffMS: 0},
			HealthCheck: &config.RouteHealthCheckConfig{Passive: config.PassiveHealthConfig{Enabled: true, FailureThreshold: 1, CooldownMS: 60000}},
		}},
	})
	if err != nil {
		t.Fatalf("failed to build resolver: %v", err)
	}

	h := NewHandler(resolver, 40*time.Millisecond)
	engine := gin.New()
	engine.NoRoute(func(c *gin.Context) { h.ServeHTTP(c) })
	gateway := httptest.NewServer(engine)
	defer gateway.Close()

	// First request contacts (and ejects) both upstreams -> 502.
	first, err := gateway.Client().Get(gateway.URL + "/events/data")
	if err != nil {
		t.Fatalf("first request failed: %v", err)
	}
	_ = first.Body.Close()
	if first.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected first request to return 502, got %d", first.StatusCode)
	}

	// Second request finds no healthy upstream -> 503 NO_HEALTHY_UPSTREAM.
	second, err := gateway.Client().Get(gateway.URL + "/events/data")
	if err != nil {
		t.Fatalf("second request failed: %v", err)
	}
	defer func() { _ = second.Body.Close() }()
	bodyBytes, err := io.ReadAll(second.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}

	if second.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503 when all upstreams unhealthy, got %d", second.StatusCode)
	}
	if !strings.Contains(string(bodyBytes), "NO_HEALTHY_UPSTREAM") {
		t.Fatalf("expected NO_HEALTHY_UPSTREAM error code, got %q", string(bodyBytes))
	}
}

func TestBreakerRejectionDoesNotEjectPassiveHealth(t *testing.T) {
	// Circuit-breaker rejections (ErrOpenState) must not feed passive-health
	// failure counters, since no request was sent to the upstream.
	//
	// Setup: one upstream at a refused port so transport errors trip the
	// breaker quickly (threshold=2). Passive-health threshold is set much
	// higher (5). In the old (buggy) code, each breaker rejection would
	// unconditionally call RecordResult(true), eventually reaching the passive
	// threshold and returning 503 NO_HEALTHY_UPSTREAM. In the fixed code only
	// real transport outcomes feed RecordResult, so we should keep getting 502
	// UPSTREAM_UNAVAILABLE (from the open breaker) and never 503.
	gin.SetMode(gin.TestMode)

	// Listen, capture the address, then immediately close — all subsequent
	// dials will get "connection refused", causing transport-level errors.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	refusedAddr := "http://" + ln.Addr().String()
	_ = ln.Close()

	resolver, err := app.NewResolver(config.Config{
		Routes: []config.RouteConfig{{
			Name:       "cb",
			Methods:    []string{"GET"},
			PathPrefix: "/cb",
			Upstream:   refusedAddr,
			CircuitBreaker: &config.RouteCircuitBreakerConfig{
				Enabled:             true,
				FailureThreshold:    2,
				OpenTimeoutMS:       5000,
				HalfOpenMaxRequests: 1,
			},
			HealthCheck: &config.RouteHealthCheckConfig{
				Passive: config.PassiveHealthConfig{
					Enabled:          true,
					FailureThreshold: 5, // requires 5 real failures to eject
					CooldownMS:       60000,
				},
			},
		}},
	})
	if err != nil {
		t.Fatalf("failed to build resolver: %v", err)
	}

	h := NewHandler(resolver, 200*time.Millisecond)
	engine := gin.New()
	engine.NoRoute(func(c *gin.Context) { h.ServeHTTP(c) })
	gateway := httptest.NewServer(engine)
	defer gateway.Close()

	// Two real transport errors open the circuit breaker (and add 2 to passive
	// health counter, still below the threshold of 5).
	for i := 0; i < 2; i++ {
		res, reqErr := gateway.Client().Get(gateway.URL + "/cb/x")
		if reqErr != nil {
			t.Fatalf("request %d failed: %v", i, reqErr)
		}
		_ = res.Body.Close()
	}

	// Send 5 more requests. The breaker is now OPEN so none reach the upstream.
	// With the bug, each call would RecordResult(true) → consecFailures reaches
	// 7 (≥5) → upstream ejected → 503 NO_HEALTHY_UPSTREAM.
	// With the fix, passive-health counter stays at 2 and we keep getting 502.
	for i := 0; i < 5; i++ {
		res, reqErr := gateway.Client().Get(gateway.URL + "/cb/x")
		if reqErr != nil {
			t.Fatalf("breaker request %d failed: %v", i, reqErr)
		}
		defer func() { _ = res.Body.Close() }()
		bodyBytes, readErr := io.ReadAll(res.Body)
		if readErr != nil {
			t.Fatalf("failed to read body: %v", readErr)
		}

		if res.StatusCode != http.StatusBadGateway {
			t.Fatalf("request %d: expected 502 from open breaker, got %d body=%q", i, res.StatusCode, bodyBytes)
		}
		if strings.Contains(string(bodyBytes), "NO_HEALTHY_UPSTREAM") {
			t.Fatalf("request %d: upstream was wrongly ejected by passive health from breaker-only rejections", i)
		}
	}
}

func testResolver(t *testing.T, upstreamURL string) *app.Resolver {
	t.Helper()
	return testResolverWithRoute(t, upstreamURL, "/events", false)
}

func testResolverWithRoute(t *testing.T, upstreamURL, pathPrefix string, trimPath bool) *app.Resolver {
	t.Helper()

	resolver, err := app.NewResolver(config.Config{
		Routes: []config.RouteConfig{{
			Name:       "events",
			Methods:    []string{"GET"},
			PathPrefix: pathPrefix,
			Upstream:   upstreamURL,
			TrimPath:   trimPath,
		}},
	})
	if err != nil {
		t.Fatalf("failed to build resolver: %v", err)
	}

	return resolver
}
