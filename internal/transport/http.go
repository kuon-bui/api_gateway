package transport

import (
	"fmt"
	"net/http"

	"api-gateway/internal/app"
	"api-gateway/internal/config"
	"api-gateway/internal/domain"
	"api-gateway/internal/middleware"
	"api-gateway/internal/proxy"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

func NewServer(cfg config.Config, logger *logrus.Logger) (*http.Server, error) {
	// gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())

	engine.Use(middleware.RequestID())
	engine.Use(middleware.PrometheusMetrics())
	engine.Use(middleware.AccessLog(logger))
	// engine.Use(middleware.JWTAuth(cfg.Security.JWT))
	engine.Use(middleware.RateLimit(cfg.RateLimit))

	engine.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	engine.GET("/readyz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})
	engine.GET("/metrics", gin.WrapH(promhttp.Handler()))

	resolver, err := app.NewResolver(cfg)
	if err != nil {
		return nil, fmt.Errorf("build route resolver: %w", err)
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
	return srv, nil
}

func WriteInternalError(c *gin.Context) {
	domain.WriteError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
}
