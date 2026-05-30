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
		if route.Balancer == nil {
			return fmt.Errorf("route %q has no upstreams configured", route.Name)
		}
		ups := route.Balancer.Upstreams()
		reachable := 0
		var lastErr error
		for _, up := range ups {
			if up.URL == nil || up.URL.Host == "" {
				lastErr = fmt.Errorf("upstream host is empty for route %q", route.Name)
				continue
			}
			conn, err := net.DialTimeout("tcp", up.URL.Host, timeout)
			if err != nil {
				lastErr = fmt.Errorf("upstream not ready for route %q (%s): %w", route.Name, up.URL.Host, err)
				continue
			}
			_ = conn.Close()
			reachable++
		}
		// A route with a load-balanced pool is considered ready as long as at least
		// one upstream is reachable (the gateway can route around the rest via
		// passive health/failover). Failing the entire gateway for a partial outage
		// would defeat the purpose of the pool.
		if reachable == 0 {
			if lastErr != nil {
				return lastErr
			}
			return fmt.Errorf("route %q has no reachable upstreams", route.Name)
		}
	}

	return nil
}
