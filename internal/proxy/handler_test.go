package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
