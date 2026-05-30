package transport

import (
	"net"
	"testing"
	"time"

	"api-gateway/internal/app"
	"api-gateway/internal/config"
)

func TestCheckUpstreamReadiness(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to open listener: %v", err)
	}
	defer func() { _ = ln.Close() }()

	resolver, err := app.NewResolver(config.Config{
		Routes: []config.RouteConfig{{
			Name:       "ok",
			Upstream:   "http://" + ln.Addr().String(),
			Methods:    []string{"GET"},
			PathPrefix: "/ok",
		}},
	})
	if err != nil {
		t.Fatalf("failed to build resolver: %v", err)
	}

	if err := checkUpstreamReadiness(resolver.Snapshot(), 200*time.Millisecond); err != nil {
		t.Fatalf("checkUpstreamReadiness() returned error: %v", err)
	}
}

func TestCheckUpstreamReadinessFailure(t *testing.T) {
	resolver, err := app.NewResolver(config.Config{
		Routes: []config.RouteConfig{{
			Name:       "bad",
			Upstream:   "http://127.0.0.1:1",
			Methods:    []string{"GET"},
			PathPrefix: "/bad",
		}},
	})
	if err != nil {
		t.Fatalf("failed to build resolver: %v", err)
	}

	if err := checkUpstreamReadiness(resolver.Snapshot(), 50*time.Millisecond); err == nil {
		t.Fatal("expected error for unreachable upstream")
	}
}

func TestCheckUpstreamReadinessToleratesPartialPoolFailure(t *testing.T) {
	// A pool with one healthy upstream and one unreachable one should pass
	// readiness — the gateway can still serve requests via failover.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to open listener: %v", err)
	}
	defer func() { _ = ln.Close() }()

	resolver, err := app.NewResolver(config.Config{
		Routes: []config.RouteConfig{{
			Name:       "pool",
			Methods:    []string{"GET"},
			PathPrefix: "/pool",
			Upstreams: []config.UpstreamConfig{
				{URL: "http://" + ln.Addr().String(), Weight: 1},
				{URL: "http://127.0.0.1:1", Weight: 1},
			},
		}},
	})
	if err != nil {
		t.Fatalf("failed to build resolver: %v", err)
	}

	if err := checkUpstreamReadiness(resolver.Snapshot(), 200*time.Millisecond); err != nil {
		t.Fatalf("expected readiness to pass with at least one reachable upstream, got: %v", err)
	}
}

func TestCheckUpstreamReadinessFailsWhenAllUnreachable(t *testing.T) {
	resolver, err := app.NewResolver(config.Config{
		Routes: []config.RouteConfig{{
			Name:       "down",
			Methods:    []string{"GET"},
			PathPrefix: "/down",
			Upstreams: []config.UpstreamConfig{
				{URL: "http://127.0.0.1:1", Weight: 1},
				{URL: "http://127.0.0.1:2", Weight: 1},
			},
		}},
	})
	if err != nil {
		t.Fatalf("failed to build resolver: %v", err)
	}

	if err := checkUpstreamReadiness(resolver.Snapshot(), 50*time.Millisecond); err == nil {
		t.Fatal("expected readiness to fail when all pool upstreams are unreachable")
	}
}
