// Package main is the entry point for the erg-server binary.
//
// @title ERG Platform API
// @version 1.0
// @description The unified backend API for ERG — a multi-tenant eLearning, content management, and automation platform.
// @contact.name ERG Support
// @contact.url https://erg.ninja/support
//
// @host localhost:8080
// @BasePath /
//
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer <token>" to authenticate.
//
// Phase 4 (task3.md): build-tag module selection + plugin system.
// This file handles server bootstrap including module registration
// and graceful shutdown coordination.
package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"

	_ "erg.ninja/pkg/plugin" // side-effect: registers build-tag modules

	docs "erg.ninja/docs" // swagger generated docs (GetSwaggerJSON, EmbeddedSwaggerUI)

	"erg.ninja/internal/middleware"
	"erg.ninja/internal/routes"
	"erg.ninja/lib/shared"
	"erg.ninja/pkg/auth"
	"erg.ninja/pkg/cache"
	"erg.ninja/pkg/compose"
	"erg.ninja/pkg/config"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/discovery"
	"erg.ninja/pkg/event"
	"erg.ninja/pkg/http/interceptors"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/queue"
	"erg.ninja/pkg/storage"
	"erg.ninja/pkg/tenant"
)

// Run bootstraps the erg-server monolith using UberFX for dependency injection.
// It wires all infrastructure providers and registers lifecycle hooks for
// graceful startup/shutdown.
func Run() error {
	fx.New(
		fx.WithLogger(func() fxevent.Logger {
			return fxevent.NopLogger
		}),
		fx.Provide(provideConfig),
		fx.Provide(provideLogger),
		fx.Provide(provideMongo),
		fx.Provide(provideRedis),
		fx.Provide(provideGORM),
		fx.Provide(provideAsynq),
		fx.Provide(provideEventBus),
		fx.Provide(provideJWTValidator),
		fx.Provide(provideR2),
		fx.Provide(provideGDrive),
		fx.Provide(provideDiscoveryFactory),
		fx.Provide(provideAsynqServer),

		// Phase 4: wire registered modules (build-tag selected)
		fx.Invoke(registerModules),

		// HTTP server + lifecycle
		fx.Invoke(registerHTTPServer),
	).Run()
	return nil
}

// registerModules prints enabled modules and sets up the plugin registry
// for use by routes.RegisterWithConfig.
func registerModules(log *logger.Logger, cfg *config.Config) {
	names := EnabledModules()
	if len(names) == 0 {
		log.Debug().Msg("server: no modules registered via build tags - using legacy wiring")
		return
	}
	log.Info().Strs("modules", names).Msg("server: build-tag modules registered")
}

// ─── Provider Constructors (moved from main.go) ───────────────────────────────

func provideConfig() (*config.Config, error) {
	cfg := config.NewDefault()
	loader := config.NewLoader(
		config.WithConfigPaths(".", "./config"),
		config.WithFileNames("application", "config"),
	)
	if err := loader.Load(cfg); err != nil {
		return nil, fmt.Errorf("config: load: %w", err)
	}
	return cfg, nil
}

func provideLogger(cfg *config.Config) *logger.Logger {
	opts := []logger.Option{
		logger.WithServiceName("erg-server"),
		logger.WithLevel(cfg.Logging.Level),
		logger.WithTimeFormat(cfg.Logging.TimeFormat),
	}
	switch strings.ToLower(strings.TrimSpace(cfg.Logging.Format)) {
	case "console", "pretty", "spring":
		opts = append(opts, logger.WithConsoleFormat())
	}
	return logger.New(opts...)
}

func provideMongo(cfg *config.Config, log *logger.Logger) (*database.MongoClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return database.NewMongoClient(ctx, cfg.MongoDB, database.WithMongoLogger(log))
}

func provideGORM(cfg *config.Config, log *logger.Logger) (*database.GORMPostgresClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dbCfg := database.PostgresConfig{
		Host:            cfg.Database.Host,
		Port:            cfg.Database.Port,
		User:            cfg.Database.User,
		Password:        cfg.Database.Password,
		Database:        cfg.Database.Name,
		SSLMode:         cfg.Database.SSLMode,
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
	}

	client, err := database.NewGORMPostgresClient(ctx, dbCfg, log)
	if err != nil {
		log.Warn().Err(err).Msg("gorm: config error or server unavailable (skipping GORM startup)")
		return nil, nil // Return nil so it doesn't break bootstrap if unused
	}
	return client, nil
}

func provideRedis(cfg *config.Config, log *logger.Logger) (*cache.RedisClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	redis, err := cache.NewRedisClient(ctx, cfg.Redis, cache.WithRedisLogger(log))
	if err != nil {
		if isDevelopment(cfg) {
			log.Warn().Err(err).Msg("redis: unavailable - continuing without Redis in development")
			return nil, nil
		}
		return nil, err
	}
	return redis, nil
}

func provideAsynq(cfg *config.Config, log *logger.Logger) (*queue.AsynqClient, error) {
	return queue.NewAsynqClient(cfg.Queue, queue.WithAsynqClientLogger(log))
}

func provideAsynqServer(cfg *config.Config, log *logger.Logger, redis *cache.RedisClient) (*queue.AsynqServer, error) {
	if !cfg.Queue.IsServer {
		return nil, nil
	}
	if isDevelopment(cfg) && redis == nil {
		log.Warn().Msg("queue: worker disabled in development because Redis is unavailable")
		return nil, nil
	}
	return queue.NewAsynqServer(cfg.Queue, queue.WithAsynqServerLogger(log))
}

func provideEventBus(cfg *config.Config, log *logger.Logger, redis *cache.RedisClient) *event.EventBus {
	opts := []event.BusOption{
		event.WithBusLogger(log),
		event.WithRedisSubscriptions(true),
	}
	if redis != nil {
		opts = append(opts, event.WithRedisBackend(redis))
		log.Info().Msg("event bus: redis pubsub enabled")
	} else {
		log.Warn().Msg("event bus: redis backend unavailable - using local event bus only")
	}
	return event.NewEventBus("erg-server", opts...)
}

func provideJWTValidator(cfg *config.Config, log *logger.Logger) (*auth.JWTValidator, error) {
	if cfg.Auth.JWTSecret == "" && cfg.Auth.JWTPublicKey == "" {
		log.Warn().Msg("auth: no jwt_secret or jwt_public_key — JWT validation disabled")
		return nil, nil
	}
	v, err := auth.NewValidatorFromConfig(struct {
		JWTSecret     string
		JWTPublicKey  string
		JWTIssuer     string
		JWTAlgorithms []string
	}{
		JWTSecret:     cfg.Auth.JWTSecret,
		JWTPublicKey:  cfg.Auth.JWTPublicKey,
		JWTIssuer:     cfg.Auth.JWTIssuer,
		JWTAlgorithms: cfg.Auth.JWTAlgorithms,
	})
	if err != nil {
		return nil, fmt.Errorf("auth: validator init: %w", err)
	}
	log.Info().Strs("algorithms", cfg.Auth.JWTAlgorithms).Msg("auth: JWT validator ready")
	return v, nil
}

func provideR2(cfg *config.Config, log *logger.Logger) (*storage.R2Client, error) {
	if cfg.R2.BucketName == "" || cfg.R2.Endpoint == "" || cfg.R2.AccessKeyID == "" || cfg.R2.SecretKey == "" {
		log.Warn().Msg("storage: R2 not configured - upload features will be limited")
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return storage.NewR2Client(ctx, storage.R2Config{
		BucketName:   cfg.R2.BucketName,
		Endpoint:     cfg.R2.Endpoint,
		AccessKeyID:  cfg.R2.AccessKeyID,
		SecretKey:    cfg.R2.SecretKey,
		PublicDomain: cfg.R2.PublicDomain,
		Region:       cfg.R2.Region,
	}, storage.WithR2Logger(log))
}

func provideGDrive(cfg *config.Config, log *logger.Logger) (*storage.GDriveClient, error) {
	if cfg.GDrive.CredentialJSON == "" {
		log.Warn().Msg("storage: Google Drive not configured - storage features will be limited")
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return storage.NewGDriveClient(ctx, cfg.GDrive.CredentialJSON, cfg.GDrive.FolderID)
}

// provideDiscoveryFactory creates a discovery-aware gRPC client factory
// when discovery is enabled in config. Returns nil when disabled (default).
func provideDiscoveryFactory(cfg *config.Config, log *logger.Logger) (*shared.Factory, error) {
	if !cfg.Discovery.Enabled {
		log.Debug().Msg("discovery: disabled — using direct gRPC addresses")
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Convert config.DiscoveryConfig → discovery.Config (different packages).
	discCfg := discovery.Config{
		Enabled: cfg.Discovery.Enabled,
		Backend: cfg.Discovery.Backend,
		Consul: discovery.ConsulCfg{
			Addr:           cfg.Discovery.Consul.Addr,
			Datacenter:     cfg.Discovery.Consul.Datacenter,
			Token:          cfg.Discovery.Consul.Token,
			HealthInterval: cfg.Discovery.Consul.HealthInterval,
		},
		DNS: discovery.DNSCfg{
			Domain: cfg.Discovery.DNS.Domain,
		},
	}
	// Convert static service entries (different type per package).
	if cfg.Discovery.Static.Services != nil {
		staticSvcs := make(map[string][]discovery.StaticServiceEntry, len(cfg.Discovery.Static.Services))
		for name, entries := range cfg.Discovery.Static.Services {
			for _, e := range entries {
				staticSvcs[name] = append(staticSvcs[name], discovery.StaticServiceEntry{
					Address:  e.Address,
					Tags:     e.Tags,
					Metadata: e.Metadata,
					Version:  e.Version,
				})
			}
		}
		discCfg.Static = discovery.StaticCfg{Services: staticSvcs}
	}
	discCfg.TTL = cfg.Discovery.TTL

	catalog, err := discovery.BuildCatalog(ctx, discCfg)
	if err != nil {
		return nil, fmt.Errorf("discovery: build catalog: %w", err)
	}
	if catalog == nil {
		log.Debug().Msg("discovery: catalog unavailable — using direct gRPC addresses")
		return nil, nil
	}

	factory := shared.NewFactory(catalog)
	log.Info().
		Str("backend", discCfg.Backend).
		Msg("discovery: factory ready")
	return factory, nil
}

// ─── HTTP Server ───────────────────────────────────────────────────────────────

func registerHTTPServer(
	lc fx.Lifecycle,
	log *logger.Logger,
	cfg *config.Config,
	mongo *database.MongoClient,
	redis *cache.RedisClient,
	bus *event.EventBus,
	queueClient *queue.AsynqClient,
	queueServer *queue.AsynqServer,
	jwtValidator *auth.JWTValidator,
	r2Client *storage.R2Client,
	gDriveClient *storage.GDriveClient,
	discoveryFactory *shared.Factory,
	gormClient *database.GORMPostgresClient,
) {
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	if err := router.SetTrustedProxies(cfg.HTTP.TrustedProxyCIDRs); err != nil {
		log.Warn().Err(err).Strs("trusted_proxy_cidrs", cfg.HTTP.TrustedProxyCIDRs).Msg("server: invalid trusted proxy CIDR config ignored by gin")
	}

	// Apply error interceptor as the outermost middleware (first in chain).
	// This ensures it catches panics and converts ergerr.E from all handlers.
	router.Use(interceptors.GinErrorInterceptor(log.Zerolog()))
	router.Use(middleware.RealIPWithTrustedProxies(cfg.HTTP.TrustedProxyCIDRs))

	// Apply CORS from layered configuration.
	router.Use(cors.New(buildCORSConfig(cfg)))

	if cfg.HTTP.RateLimit.Enabled {
		router.Use(middleware.RateLimit(redis, middleware.RateLimitConfig{
			RequestsPerMinute: cfg.HTTP.RateLimit.RequestsPerMinute,
			Burst:             cfg.HTTP.RateLimit.Burst,
			SkipLoopback:      !strings.EqualFold(cfg.App.Env, "production"),
		}))
	}

	// ── Swagger & Scalar UI (Registered Early) ──────────────────────────────────
	router.GET("/swagger/*path", gin.WrapH(http.StripPrefix("/swagger", docs.EmbeddedSwaggerUI())))
	router.GET("/reference", func(c *gin.Context) {
		html := `
		<!doctype html>
		<html>
		  <head>
			<title>ERG API Reference</title>
			<meta charset="utf-8" />
			<meta name="viewport" content="width=device-width, initial-scale=1" />
			<style> body { margin: 0; } </style>
		  </head>
		  <body>
			<script id="api-reference" data-url="/swagger/doc.json" data-configuration='{"theme": "purple"}'></script>
			<script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
		  </body>
		</html>`
		c.Data(200, "text/html; charset=utf-8", []byte(html))
	})
	router.GET("/swagger", func(c *gin.Context) { c.Redirect(301, "/swagger/") })

	// ── Build tenant-aware MongoDB client ─────────────────────────────────
	var tenantMongo *tenant.TenantMongoClient
	if cfg.Tenant.Enabled {
		mode := tenant.IsolationMode(cfg.Tenant.Isolation)
		if mode != tenant.IsolationCollection && mode != tenant.IsolationField {
			mode = tenant.IsolationCollection
		}
		tenantMongo = tenant.NewTenantMongoClient(
			mongo, mode, defaultTenantID(cfg),
		)
		log.Info().Str("isolation", cfg.Tenant.Isolation).Msg("server: multi-tenancy ENABLED")
	} else {
		log.Debug().Msg("server: multi-tenancy disabled (single-tenant mode)")
	}

	deps := &routes.Deps{
		Mongo:             mongo,
		Redis:             redis,
		Bus:               bus,
		Log:               log,
		Cfg:               cfg,
		Queue:             queueClient,
		QueueServer:       queueServer,
		JWTValidator:      jwtValidator,
		TenantMongoClient: tenantMongo,
		DiscoveryFactory:  discoveryFactory,
		R2:                r2Client,
		GDrive:            gDriveClient,
		GORMClient:        gormClient,
	}

	// ── Phase 5: Compose engine (config-driven composition) ─────────────────────
	// Try to load deploy.yaml. If it exists and compose is enabled, use the
	// compose engine for declarative module wiring. Otherwise fall back to
	// the legacy routes.Register() approach.
	var stopFns []func(context.Context)
	manifestPath := cfg.Compose.DeployManifestPath
	if manifestPath == "" {
		manifestPath = "deploy.yaml"
	}
	manifest, err := compose.Load(manifestPath, cfg)
	if err == nil && cfg.Compose.Enabled {
		log.Info().
			Int("services", len(compose.EnabledServices(manifest))).
			Msg("server: compose engine active")
		engine := compose.NewComposeEngine(deps)
		order, stops, bootstrapErr := engine.Bootstrap(context.Background(), router, manifest)
		if bootstrapErr != nil {
			log.Error().Err(bootstrapErr).Msg("server: compose bootstrap failed, falling back to legacy wiring")
		} else {
			stopFns = stops
			for _, svc := range order {
				log.Info().Str("service", svc.Name).Msg("server: compose service registered")
			}
		}
	} else {
		if err != nil && !errors.Is(err, compose.ErrNoDeployManifest) && !errors.Is(err, os.ErrNotExist) {
			log.Warn().Err(err).Str("path", manifestPath).Msg("server: deploy manifest load failed, using legacy wiring")
		} else {
			log.Debug().Msg("server: compose disabled — using legacy routes.Register()")
		}
		stopFns = routes.Register(router, deps)
	}

	log.Info().Msg("server: swagger UI ready")
	log.Info().Msg("server: scalar UI ready")

	addr := fmt.Sprintf("%s:%d", cfg.App.Host, cfg.App.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  cfg.App.ReadTimeout,
		WriteTimeout: cfg.App.WriteTimeout,
		IdleTimeout:  cfg.App.IdleTimeout,
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			log.Info().Str("addr", addr).Msg("erg-server: HTTP server starting")
			listener, err := net.Listen("tcp", addr)
			if err != nil {
				log.Error().Err(err).Str("addr", addr).Msg("erg-server: HTTP server bind failed")
				return err
			}

			port := cfg.App.Port
			if tcpAddr, ok := listener.Addr().(*net.TCPAddr); ok {
				port = tcpAddr.Port
			}
			log.Info().
				Str("addr", listener.Addr().String()).
				Int("port", port).
				Str("url", fmt.Sprintf("http://%s:%d", displayHost(cfg.App.Host), port)).
				Msg("erg-server: HTTP server started")

			go func() {
				if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
					log.Error().Err(err).Msg("erg-server: HTTP server serve error")
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			log.Info().Msg("erg-server: shutting down gracefully")
			if err := srv.Shutdown(ctx); err != nil {
				log.ErrorContext(ctx).Err(err).Msg("erg-server: HTTP server shutdown")
			}
			for _, stop := range stopFns {
				stop(ctx)
			}
			if queueServer != nil {
				queueServer.Shutdown()
			}
			bus.UnsubscribeAll()
			_ = mongo.Close(ctx)
			if redis != nil {
				_ = redis.Close()
			}
			if queueClient != nil {
				_ = queueClient.Close()
			}
			_ = log.Sync()
			log.Info().Msg("erg-server: stopped")
			return nil
		},
	})
}

func displayHost(host string) string {
	host = strings.TrimSpace(host)
	switch host {
	case "", "0.0.0.0", "::":
		return "localhost"
	default:
		return host
	}
}

func isDevelopment(cfg *config.Config) bool {
	if cfg == nil {
		return true
	}
	env := strings.TrimSpace(strings.ToLower(cfg.App.Env))
	return env == "" || env == "development" || env == "dev" || env == "local"
}

func defaultTenantID(cfg *config.Config) string {
	if cfg.Tenant.DefaultID != "" {
		return cfg.Tenant.DefaultID
	}
	return "default"
}

func buildCORSConfig(cfg *config.Config) cors.Config {
	c := cfg.HTTP.CORS
	methods := c.AllowedMethods
	if len(methods) == 0 {
		methods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	}
	headers := c.AllowedHeaders
	if len(headers) == 0 {
		headers = []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Tenant-ID", "X-Request-ID", "X-Trace-ID"}
	}
	exposed := c.ExposedHeaders
	if len(exposed) == 0 {
		exposed = []string{"X-Request-ID", "X-Trace-ID"}
	}
	maxAge := time.Duration(c.MaxAge) * time.Second
	if maxAge <= 0 {
		maxAge = 12 * time.Hour
	}

	corsCfg := cors.Config{
		AllowMethods:     methods,
		AllowHeaders:     headers,
		ExposeHeaders:    exposed,
		AllowCredentials: c.AllowCredentials,
		MaxAge:           maxAge,
	}

	for _, origin := range c.AllowedOrigins {
		if origin == "*" {
			corsCfg.AllowAllOrigins = true
			corsCfg.AllowCredentials = false
			return corsCfg
		}
	}
	corsCfg.AllowOrigins = c.AllowedOrigins
	return corsCfg
}

// ─── Legacy Lifecycle Hooks (kept for compatibility) ────────────────────────────

func onStart(
	log *logger.Logger,
	mongo *database.MongoClient,
	redis *cache.RedisClient,
	_ *queue.AsynqClient,
) error {
	// Warm-up check: ping MongoDB and Redis.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := mongo.Ping(ctx); err != nil {
		log.Warn().Err(err).Msg("mongo: ping failed on startup")
	}
	if redis != nil {
		_ = redis.Ping(ctx)
	}
	log.Info().Msg("erg-server: startup checks complete")
	return nil
}

func onStop(
	log *logger.Logger,
	mongo *database.MongoClient,
	redis *cache.RedisClient,
	asynq *queue.AsynqClient,
	bus *event.EventBus,
) error {
	log.Info().Msg("erg-server: graceful shutdown initiated")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if asynq != nil {
		_ = asynq.Close()
	}
	bus.UnsubscribeAll()
	_ = mongo.Close(ctx)
	if redis != nil {
		_ = redis.Close()
	}
	_ = log.Sync()
	log.Info().Msg("erg-server: shutdown complete")
	return nil
}

// Handle OS signals for graceful shutdown when not using fx lifecycle.
func handleSignals(srv *http.Server, log *logger.Logger) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Info().Str("signal", sig.String()).Msg("erg-server: shutdown signal received")
}
