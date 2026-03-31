// Package main is the entry point for the bot-service.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"erg.ninja/pkg/config"
	"erg.ninja/pkg/event"
	"erg.ninja/pkg/http"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/telemetry"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load configuration.
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger.
	log := logger.New(
		logger.WithServiceName("bot-service"),
		logger.WithLevel(cfg.Logging.Level),
		logger.WithTimeFormat(cfg.Logging.TimeFormat),
	)
	defer func() { _ = log.Sync() }()

	// Initialize OpenTelemetry.
	tp, err := telemetry.NewTracerProvider(ctx, cfg.Telemetry, log)
	if err != nil {
		log.Warn(ctx).Err(err).Msg("telemetry: failed to init tracer provider, continuing without tracing")
	}
	if tp != nil {
		defer func() { _ = tp.Shutdown(ctx) }()
	}

	// Initialize event bus (local + Redis).
	bus := event.NewEventBus("bot-service", event.WithBusLogger(log))

	// Build HTTP server.
	server := http.NewServer(cfg.HTTP, log)

	// Register routes.
	registerRoutes(server, bus, log)

	// Mount health and metrics.
	server.MountHealthRoutes()
	server.MountMetrics()

	// Start HTTP server in background.
	go func() {
		addr := fmt.Sprintf("%s:%d", cfg.HTTP.Host, cfg.HTTP.Port)
		log.Info(ctx).Str("addr", addr).Msg("bot-service: starting HTTP server")
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			log.Error(ctx).Err(err).Msg("bot-service: HTTP server error")
		}
	}()

	// Graceful shutdown.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		log.Info(ctx).Str("signal", sig.String()).Msg("bot-service: received shutdown signal")
	case <-ctx.Done():
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, cfg.App.ShutdownTime)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error(shutdownCtx).Err(err).Msg("bot-service: HTTP server shutdown error")
	}

	log.Info(ctx).Msg("bot-service: stopped")
}

func loadConfig() (*config.Config, error) {
	cfg := config.NewDefault()
	loader := config.NewLoader(
		config.WithConfigPaths("."),
		config.WithFileName("config"),
	)
	if err := loader.Load(cfg); err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	return cfg, nil
}

func registerRoutes(server *http.Server, bus *event.EventBus, log *logger.Logger) {
	server.Get("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"service":"bot-service","status":"ok"}`))
	})
}
