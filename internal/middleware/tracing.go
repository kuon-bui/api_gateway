package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

func Tracing(serviceName string) gin.HandlerFunc {
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		serviceName = "api-gateway"
	}
	return otelgin.Middleware(serviceName)
}
