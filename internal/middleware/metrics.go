package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	requestTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_http_requests_total",
			Help: "Total number of HTTP requests handled by the gateway.",
		},
		[]string{"method", "route", "status"},
	)

	requestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gateway_http_request_duration_seconds",
			Help:    "HTTP request latency in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "route", "status"},
	)
)

func PrometheusMetrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		status := strconv.Itoa(c.Writer.Status())
		method := c.Request.Method
		route := metricRouteLabel(c)

		requestTotal.WithLabelValues(method, route, status).Inc()
		requestDuration.WithLabelValues(method, route, status).Observe(time.Since(start).Seconds())
	}
}

func metricRouteLabel(c *gin.Context) string {
	routeName, _ := c.Get("route_name")
	return resolveMetricRouteLabel(routeName, c.FullPath(), c.Request.URL.Path)
}

func resolveMetricRouteLabel(routeName any, fullPath, requestPath string) string {
	if route, ok := routeName.(string); ok && route != "" {
		return route
	}

	if fullPath != "" {
		return fullPath
	}

	if requestPath == "/healthz" || requestPath == "/readyz" || requestPath == "/metrics" {
		return requestPath
	}

	return "unmatched"
}
