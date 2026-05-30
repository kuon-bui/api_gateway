package transport

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api-gateway/internal/config"

	"github.com/sirupsen/logrus"
)

func TestAdminRoutesRequiresAPIKey(t *testing.T) {
	cfg := config.Config{
		Server: config.ServerConfig{Port: 8080, ReadTimeoutMS: 1000, WriteTimeoutMS: 1000, IdleTimeoutMS: 1000},
		Proxy:  config.ProxyConfig{TimeoutMS: 1000},
		Admin:  config.AdminConfig{Enabled: true, APIKey: "secret-admin-key"},
		Routes: []config.RouteConfig{{
			Name:       "events",
			Methods:    []string{"GET"},
			PathPrefix: "/events",
			Upstream:   "http://localhost:9001",
		}},
	}

	srv, _, err := NewServer(cfg, logrus.New())
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}

	testServer := httptest.NewServer(srv.Handler)
	defer testServer.Close()

	resp, err := http.Get(testServer.URL + "/admin/routes")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", resp.StatusCode)
	}
}

func TestAdminRoutesReturnsConfiguredRoutes(t *testing.T) {
	cfg := config.Config{
		Server: config.ServerConfig{Port: 8080, ReadTimeoutMS: 1000, WriteTimeoutMS: 1000, IdleTimeoutMS: 1000},
		Proxy:  config.ProxyConfig{TimeoutMS: 1000},
		Admin:  config.AdminConfig{Enabled: true, APIKey: "secret-admin-key"},
		Routes: []config.RouteConfig{{
			Name:       "events",
			Methods:    []string{"GET"},
			PathPrefix: "/events",
			Upstream:   "http://localhost:9001",
		}},
	}

	srv, _, err := NewServer(cfg, logrus.New())
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}

	testServer := httptest.NewServer(srv.Handler)
	defer testServer.Close()

	req, err := http.NewRequest(http.MethodGet, testServer.URL+"/admin/routes", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("X-Admin-Key", "secret-admin-key")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed reading body: %v", err)
	}
	body := string(bodyBytes)
	if !strings.Contains(body, "\"events\"") {
		t.Fatalf("expected routes payload to contain route name, got %s", body)
	}
	if !strings.Contains(body, "\"upstreams\"") || !strings.Contains(body, "http://localhost:9001") {
		t.Fatalf("expected routes payload to expose upstreams, got %s", body)
	}
	if !strings.Contains(body, "\"healthy\"") {
		t.Fatalf("expected routes payload to expose upstream health, got %s", body)
	}
}
