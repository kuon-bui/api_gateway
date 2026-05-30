package transport

import (
	"fmt"
	"net"
	"time"

	"api-gateway/internal/domain"
)

func checkUpstreamReadiness(routes []domain.Route, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = time.Second
	}

	for _, route := range routes {
		if route.Upstream == nil || route.Upstream.Host == "" {
			return fmt.Errorf("route %q upstream host is empty", route.Name)
		}

		conn, err := net.DialTimeout("tcp", route.Upstream.Host, timeout)
		if err != nil {
			return fmt.Errorf("upstream not ready for route %q (%s): %w", route.Name, route.Upstream.Host, err)
		}
		_ = conn.Close()
	}

	return nil
}
