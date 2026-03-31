// Package main is the entry point for the trending-service.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"erg.ninja/pkg/config"
	"erg.ninja/pkg/event"
	httphandler "erg.ninja/pkg/http"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/telemetry"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	log := logger.New(
		logger.WithServiceName("trending-service"),
		logger.WithLevel(cfg.Logging.Level),
	)
	defer func() { _ = log.Sync() }()

	tp, err := telemetry.NewTracerProvider(ctx, cfg.Telemetry, log)
	if err != nil {
		log.Warn(ctx).Err(err).Msg("telemetry: init failed")
	}
	if tp != nil {
		defer func() { _ = tp.Shutdown(ctx) }()
	}

	bus := event.NewEventBus("trending-service", event.WithBusLogger(log))
	_ = bus

	server := httphandler.NewServer(cfg.HTTP, log)
	server.MountHealthRoutes()
	server.MountMetrics()

	server.Get("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"service":"trending-service","status":"ok"}`))
	})

	go func() {
		addr := fmt.Sprintf("%s:%d", cfg.HTTP.Host, cfg.HTTP.Port)
		log.Info(ctx).Str("addr", addr).Msg("trending-service: starting")
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			log.Error(ctx).Err(err).Msg("trending-service: server error")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutdownCtx, cancel := context.WithTimeout(ctx, cfg.App.ShutdownTime)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
	log.Info(ctx).Msg("trending-service: stopped")
}

func loadConfig() (*config.Config, error) {
	cfg := config.NewDefault()
	loader := config.NewLoader(config.WithConfigPaths("."))
	if err := loader.Load(cfg); err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	return cfg, nil
}
