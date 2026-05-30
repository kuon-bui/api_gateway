package proxy

import (
	"errors"
	"io"
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
	defer res.Body.Close()
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
	defer res.Body.Close()
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
	defer res.Body.Close()

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
	defer res.Body.Close()

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
	defer res.Body.Close()

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
