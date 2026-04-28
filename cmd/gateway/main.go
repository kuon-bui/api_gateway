package main

import (
	"context"
	"errors"
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
