package transport

import (
	"fmt"
	"net/http"
	"sort"

	"api-gateway/internal/app"
	"api-gateway/internal/config"
	"api-gateway/internal/domain"
	"api-gateway/internal/middleware"
	"api-gateway/internal/proxy"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

func NewServer(cfg config.Config, logger *logrus.Logger) (*http.Server, *app.Resolver, error) {
	resolver, err := app.NewResolver(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("build route resolver: %w", err)
	}

	// gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())
	if cfg.Telemetry.Enabled {
		engine.Use(middleware.Tracing(cfg.Telemetry.ServiceName))
	}

	engine.Use(middleware.RequestID())
	engine.Use(middleware.PrometheusMetrics())
	engine.Use(middleware.AccessLog(logger))
	// engine.Use(middleware.JWTAuth(cfg.Security.JWT))
	engine.Use(middleware.RateLimit(cfg.RateLimit, resolver))

	engine.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	engine.GET("/readyz", func(c *gin.Context) {
		if err := checkUpstreamReadiness(resolver.Snapshot(), cfg.ProxyTimeout()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "not_ready",
				"reason": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})
	engine.GET("/metrics", gin.WrapH(promhttp.Handler()))
	if cfg.Admin.Enabled {
		registerAdminRoutes(engine, cfg.Admin, resolver)
	}

	proxyHandler := proxy.NewHandler(resolver, cfg.ProxyTimeout())

	engine.NoRoute(func(c *gin.Context) {
		if c.Request.URL.Path == "/healthz" || c.Request.URL.Path == "/readyz" || c.Request.URL.Path == "/metrics" {
			return
		}

		proxyHandler.ServeHTTP(c)
	})

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      engine,
		ReadTimeout:  cfg.ReadTimeout(),
		WriteTimeout: cfg.WriteTimeout(),
		IdleTimeout:  cfg.IdleTimeout(),
	}
	return srv, resolver, nil
}

func WriteInternalError(c *gin.Context) {
	domain.WriteError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
}

func registerAdminRoutes(engine *gin.Engine, adminCfg config.AdminConfig, resolver *app.Resolver) {
	admin := engine.Group("/admin")
	admin.Use(func(c *gin.Context) {
		if c.GetHeader("X-Admin-Key") != adminCfg.APIKey {
			domain.WriteError(c, http.StatusUnauthorized, "ADMIN_UNAUTHORIZED", "Admin access denied")
			c.Abort()
			return
		}
		c.Next()
	})

	admin.GET("/routes", func(c *gin.Context) {
		routes := resolver.Snapshot()
		payload := make([]gin.H, 0, len(routes))
		for _, route := range routes {
			methods := make([]string, 0, len(route.Methods))
			for method := range route.Methods {
				methods = append(methods, method)
			}
			sort.Strings(methods)

			payload = append(payload, gin.H{
				"name":        route.Name,
				"methods":     methods,
				"path_prefix": route.PathPrefix,
				"upstream":    route.Upstream.String(),
				"trim_path":   route.TrimPath,
			})
		}

		c.JSON(http.StatusOK, gin.H{"routes": payload})
	})
}
