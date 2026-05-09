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
	"api-gateway/internal/transport"

	"github.com/sirupsen/logrus"
)

func main() {
	logger := initLogger()

	cfg, err := config.Load("")
	if err != nil {
		logger.WithError(err).Fatal("failed to load config")
	}

	srv, err := transport.NewServer(cfg, logger)
	if err != nil {
		logger.WithError(err).Fatal("failed to build server")
	}

	mode := "debug"
	if ginMode := os.Getenv("GIN_MODE"); ginMode == "release" {
		mode = "release"
	}
	printBanner(cfg, mode)

	go func() {
		logger.WithField("addr", srv.Addr).Info("gateway starting")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.WithError(err).Fatal("server failed")
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
