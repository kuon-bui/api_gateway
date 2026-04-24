package middleware

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func AccessLog(logger *logrus.Logger) gin.HandlerFunc {
	// Dedicated logger with custom access log formatter.
	accessLogger := logrus.New()
	accessLogger.SetOutput(os.Stdout)
	accessLogger.SetLevel(logger.Level)
	accessLogger.SetFormatter(&AccessLogFormatter{})

	return func(c *gin.Context) {
		start := time.Now()
		if isWebSocketUpgradeRequest(c) {
			accessLogger.WithFields(logrus.Fields{
				"request_id": c.GetHeader("X-Request-ID"),
				"method":     c.Request.Method,
				"path":       c.Request.URL.Path,
				"status":     http.StatusSwitchingProtocols,
				"latency_ms": int64(0),
				"route_name": "",
				"upstream":   "",
				"protocol":   "websocket",
			}).Info("websocket request started")
		}
		c.Next()

		latency := time.Since(start)
		routeName, _ := c.Get("route_name")
		upstream, _ := c.Get("upstream")
		trimmedPath, _ := c.Get("trimmed_path")
		routeNameStr := toString(routeName)
		pathDisplay := c.Request.URL.Path
		if routeNameStr != "" {
			trimmed := toString(trimmedPath)
			if trimmed == "" {
				trimmed = c.Request.URL.Path
			}
			pathDisplay = fmt.Sprintf("%s => %s", routeNameStr, trimmed)
		}
		status := c.Writer.Status()
		if statusOverride, ok := c.Get("access_status"); ok {
			if code, ok := statusOverride.(int); ok {
				status = code
			}
		}

		fields := logrus.Fields{
			"request_id": c.GetHeader("X-Request-ID"),
			"method":     c.Request.Method,
			"path":       pathDisplay,
			"status":     status,
			"latency_ms": latency.Milliseconds(),
			"route_name": routeNameStr,
			"upstream":   toString(upstream),
		}
		if isWebSocket, ok := c.Get("is_websocket"); ok {
			if ws, ok := isWebSocket.(bool); ok && ws {
				fields["protocol"] = "websocket"
			}
		}

		accessLogger.WithFields(fields).Info("request completed")
	}
}

func isWebSocketUpgradeRequest(c *gin.Context) bool {
	return strings.EqualFold(c.GetHeader("Upgrade"), "websocket") &&
		strings.Contains(strings.ToLower(c.GetHeader("Connection")), "upgrade")
}

func toString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
