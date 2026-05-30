package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"api-gateway/internal/config"
	"api-gateway/internal/telemetry"
	"api-gateway/internal/transport"

	"github.com/sirupsen/logrus"
)

func main() {
	logger := initLogger()

	configPath := os.Getenv("GATEWAY_CONFIG")
	cfg, resolvedConfigPath, err := config.LoadWithPath(configPath)
	if err != nil {
		logger.WithError(err).Fatal("failed to load config")
	}

	shutdownTracing, err := telemetry.Init(context.Background(), cfg.Telemetry)
	if err != nil {
		logger.WithError(err).Fatal("failed to initialize telemetry")
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTracing(ctx); err != nil {
			logger.WithError(err).Warn("failed to shutdown telemetry")
		}
	}()

	srv, resolver, err := transport.NewServer(cfg, logger)
	if err != nil {
		logger.WithError(err).Fatal("failed to build server")
	}

	stopWatch, err := config.Watch(resolvedConfigPath, func(next config.Config) {
		if err := resolver.Reload(next); err != nil {
			logger.WithError(err).Warn("config reload failed")
			return
		}
		logger.WithField("routes", len(next.Routes)).Info("config reloaded")
	})
	if err != nil {
		logger.WithError(err).Warn("failed to start config watcher")
	} else {
		defer stopWatch()
	}

	mode := "debug"
	if ginMode := os.Getenv("GIN_MODE"); ginMode == "release" {
		mode = "release"
	}
	printBanner(cfg, mode)

	go func() {
		logger.WithField("addr", srv.Addr).Info("gateway starting")
		var serveErr error
		if cfg.Server.TLS.Enabled {
			serveErr = srv.ListenAndServeTLS(cfg.Server.TLS.CertFile, cfg.Server.TLS.KeyFile)
		} else {
			serveErr = srv.ListenAndServe()
		}
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			logger.WithError(serveErr).Fatal("server failed")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.WithError(err).Warn("graceful shutdown failed")
	}
	logger.Info("gateway stopped")
}

func printBanner(cfg config.Config, mode string) {
	fmt.Println()
	fmt.Println("┌─────────────────────────────────────────┐")
	fmt.Println("│              API GATEWAY                │")
	fmt.Println("├─────────────────────────────────────────┤")
	fmt.Printf("│  Port      %-29d│\n", cfg.Server.Port)
	fmt.Printf("│  Mode      %-29s│\n", mode)
	fmt.Printf("│  Routes    %-29d│\n", len(cfg.Routes))
	fmt.Println("└─────────────────────────────────────────┘")
	fmt.Println()
}

func initLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
		ForceColors:     true,
	})
	logger.SetOutput(os.Stdout)
	logger.SetLevel(logrus.InfoLevel)
	return logger
}
