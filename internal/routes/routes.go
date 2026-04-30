// Package routes registers all application modules onto the gin router.
// It is the single place where modules are mounted, mirroring NestJS AppModule imports.
package routes

import (
	"context"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"erg.ninja/internal/dto/response"
	"erg.ninja/internal/middleware"
	"erg.ninja/internal/modules/access_control"
	"erg.ninja/internal/modules/ai_content"
	"erg.ninja/internal/modules/analytics"
	audit "erg.ninja/internal/modules/audit"
	authmodule "erg.ninja/internal/modules/auth"
	"erg.ninja/internal/modules/bot"
	"erg.ninja/internal/modules/courses"
	"erg.ninja/internal/modules/crawler"
	"erg.ninja/internal/modules/documents"
	"erg.ninja/internal/modules/elearning"
	"erg.ninja/internal/modules/lms"
	"erg.ninja/internal/modules/menus"
	"erg.ninja/internal/modules/notifications"
	"erg.ninja/internal/modules/operations"
	"erg.ninja/internal/modules/pages"
	"erg.ninja/internal/modules/posts"
	"erg.ninja/internal/modules/profiles"
	"erg.ninja/internal/modules/public_disclosure"
	"erg.ninja/internal/modules/recruitment"
	"erg.ninja/internal/modules/reviews"
	"erg.ninja/internal/modules/seo"
	"erg.ninja/internal/modules/sessions"
	"erg.ninja/internal/modules/sitemap"
	"erg.ninja/internal/modules/trending"
	"erg.ninja/internal/modules/users"
	"erg.ninja/internal/persistence/postgrescore"
	"erg.ninja/lib/shared"
	"erg.ninja/pkg/auth"
	"erg.ninja/pkg/cache"
	"erg.ninja/pkg/config"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/event"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/queue"
	"erg.ninja/pkg/storage"
	"erg.ninja/pkg/tenant"
)

// Deps holds shared dependencies passed to all modules.
type Deps struct {
	Mongo             *database.MongoClient
	Redis             *cache.RedisClient
	Bus               *event.EventBus
	Log               *logger.Logger
	Cfg               *config.Config
	Queue             *queue.AsynqClient
	QueueServer       *queue.AsynqServer
	JWTValidator      *auth.JWTValidator
	TenantMongoClient *tenant.TenantMongoClient
	DiscoveryFactory  *shared.Factory
	R2                *storage.R2Client
	GDrive            *storage.GDriveClient
	GORMClient        *database.GORMPostgresClient
}

// defaultTenantID returns the configured default tenant or "default".
func defaultTenantID(cfg *config.Config) string {
	if cfg.Tenant.DefaultID != "" {
		return cfg.Tenant.DefaultID
	}
	return "default"
}

// Register wires all modules and registers their routes onto the given router.
func Register(r *gin.Engine, deps *Deps) []func(context.Context) {
	return RegisterWithConfig(r, deps, nil)
}

// RegisterWithConfig registers modules with optional plugin configuration.
func RegisterWithConfig(r *gin.Engine, deps *Deps, pluginCfg *PluginConfig) []func(context.Context) {
	var tenantMongo *tenant.TenantMongoClient
	if deps.Cfg.Tenant.Enabled {
		mode := tenant.IsolationMode(deps.Cfg.Tenant.Isolation)
		if mode != tenant.IsolationCollection && mode != tenant.IsolationField {
			mode = tenant.IsolationCollection
		}
		tenantMongo = tenant.NewTenantMongoClient(deps.Mongo, mode, defaultTenantID(deps.Cfg))
		deps.TenantMongoClient = tenantMongo
	}

	if deps.Cfg.Tenant.Enabled {
		r.Use(tenant.TenantMiddlewareGin(true, true, defaultTenantID(deps.Cfg)))
		deps.Log.Info().Str("isolation", deps.Cfg.Tenant.Isolation).Msg("routes: multi-tenancy ENABLED")
	} else {
		r.Use(tenant.TenantMiddlewareGin(false, false, defaultTenantID(deps.Cfg)))
		deps.Log.Debug().Msg("routes: multi-tenancy disabled (single-tenant mode)")
	}

	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	if pluginCfg != nil && len(pluginCfg.EnabledModules) > 0 {
		wiring, err := BuildFromRegistry(deps, pluginCfg.EnabledModules)
		if err != nil {
			deps.Log.Error().Err(err).Msg("routes: plugin registry wiring failed, falling back to legacy")
		} else {
			for _, mod := range wiring.Modules {
				deps.Log.Info().Str("module", mod.Name()).Msg("routes: registering plugin module")
				mod.RegisterRoutes(r)
			}
			return wiring.StopFuncs
		}
	}

	RegisterHealthRoutes(r, deps)
	return legacyRegister(r, deps)
}

// RegisterHealthRoutes registers global liveness and readiness probes.
func RegisterHealthRoutes(r *gin.Engine, deps *Deps) {
	startTime := time.Now()

	r.GET("/api/health", func(c *gin.Context) {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		response.SuccessGin(c, gin.H{
			"status":    "ok",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"uptime":    int64(time.Since(startTime).Seconds()),
			"instance":  "erg-go-monolith",
			"memory": gin.H{
				"used":  m.Alloc / 1024 / 1024,
				"total": m.Sys / 1024 / 1024,
				"unit":  "MB",
			},
			"version": "1.0.0",
		})
	})

	r.GET("/api/ready", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		checks := gin.H{
			"app":   true,
			"db":    deps.Mongo.Ping(ctx) == nil,
			"redis": deps.Redis.Ping(ctx) == nil,
		}

		status := "ready"
		for _, ok := range checks {
			if b, ok := ok.(bool); ok && !b {
				status = "degraded"
				break
			}
		}

		response.SuccessGin(c, gin.H{
			"status":    status,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"checks":    checks,
		})
	})
}

// legacyRegister wires modules without the plugin registry.
func legacyRegister(r *gin.Engine, deps *Deps) []func(context.Context) {
	var stops []func(context.Context)

	if deps.GORMClient != nil && deps.Cfg.Database.AutoMigrate {
		if err := postgrescore.AutoMigrate(deps.GORMClient.DB()); err != nil {
			deps.Log.Warn().Err(err).Msg("routes: postgres core automigrate failed")
		} else {
			deps.Log.Info().Msg("routes: postgres core schema ready")
		}
	} else if deps.GORMClient != nil {
		deps.Log.Info().Msg("routes: postgres core automigrate skipped; run cmd/db-migrate when schema changes")
	}

	accessControlModule := access_control.NewModule(access_control.Deps{
		Mongo:             deps.Mongo,
		GORMClient:        deps.GORMClient,
		Redis:             deps.Redis,
		Log:               deps.Log,
		Cfg:               deps.Cfg,
		JWTValidator:      deps.JWTValidator,
		TenantMongoClient: deps.TenantMongoClient,
	})
	accessControlModule.Setup()
	if deps.GORMClient != nil && deps.Mongo != nil && deps.Cfg.Database.RunBackfills {
		backfillTimeout := deps.Cfg.Database.MigrationTimeout
		if backfillTimeout <= 0 {
			backfillTimeout = 2 * time.Minute
		}
		backfillCtx, cancel := context.WithTimeout(context.Background(), backfillTimeout)
		defer cancel()
		if _, err := postgrescore.BackfillLegacyAuthFromMongo(
			backfillCtx,
			deps.GORMClient.DB(),
			deps.Mongo,
			deps.Log,
			defaultTenantID(deps.Cfg),
		); err != nil {
			deps.Log.Warn().Err(err).Msg("routes: legacy auth backfill failed")
		}
		if _, err := postgrescore.BackfillLegacyRecruitmentFromMongo(
			backfillCtx,
			deps.GORMClient.DB(),
			deps.Mongo,
			deps.Log,
		); err != nil {
			deps.Log.Warn().Err(err).Msg("routes: legacy recruitment backfill failed")
		}
	} else if deps.GORMClient != nil && deps.Mongo != nil {
		deps.Log.Info().Msg("routes: legacy postgres backfills skipped; run cmd/db-migrate -backfill explicitly")
	}

	opsModule := operations.NewModule(operations.Deps{
		Mongo:             deps.Mongo,
		GORMClient:        deps.GORMClient,
		Redis:             deps.Redis,
		Log:               deps.Log,
		Cfg:               deps.Cfg,
		JWTValidator:      deps.JWTValidator,
		TenantMongoClient: deps.TenantMongoClient,
	})
	opsModule.Setup()
	r.Use(middleware.FirewallMiddleware(opsModule.Service()))

	accessControlModule.RegisterRoutes(r)
	stops = append(stops, func(ctx context.Context) {
		if err := accessControlModule.Stop(ctx); err != nil {
			deps.Log.WarnContext(ctx).Err(err).Msg("routes: access_control module stop failed")
		}
	})

	authMod := authmodule.NewModule(authmodule.Deps{
		Mongo:             deps.Mongo,
		GORMClient:        deps.GORMClient,
		Redis:             deps.Redis,
		Log:               deps.Log,
		Cfg:               deps.Cfg,
		JWTValidator:      deps.JWTValidator,
		TenantMongoClient: deps.TenantMongoClient,
		AC:                accessControlModule.Service(),
	})
	_ = authMod.Setup()
	authMod.RegisterRoutes(r)
	stops = append(stops, func(ctx context.Context) {
		_ = authMod.Stop(ctx)
	})

	notifModule := notifications.NewModule(notifications.Deps{
		Mongo:             deps.Mongo,
		Redis:             deps.Redis,
		Bus:               deps.Bus,
		Log:               deps.Log,
		Cfg:               deps.Cfg,
		JWTValidator:      deps.JWTValidator,
		TenantMongoClient: deps.TenantMongoClient,
	})
	notifModule.Setup()
	notifModule.RegisterRoutes(r)
	stops = append(stops, func(ctx context.Context) {
		if err := notifModule.Stop(ctx); err != nil {
			deps.Log.WarnContext(ctx).Err(err).Msg("routes: notifications module stop failed")
		}
	})

	aiModule := ai_content.NewModule(ai_content.Deps{
		Mongo:             deps.Mongo,
		Redis:             deps.Redis,
		Log:               deps.Log,
		Cfg:               deps.Cfg,
		TenantMongoClient: deps.TenantMongoClient,
	})
	aiModule.Setup()
	aiModule.RegisterRoutes(r)
	stops = append(stops, func(ctx context.Context) {
		if err := aiModule.Stop(ctx); err != nil {
			deps.Log.WarnContext(ctx).Err(err).Msg("routes: ai_content module stop failed")
		}
	})

	crawlerModule := crawler.NewModule(crawler.Deps{
		Mongo:             deps.Mongo,
		GORMClient:        deps.GORMClient,
		Redis:             deps.Redis,
		Bus:               deps.Bus,
		Log:               deps.Log,
		Cfg:               deps.Cfg,
		Queue:             deps.Queue,
		TenantMongoClient: deps.TenantMongoClient,
	})
	crawlerModule.Setup()
	crawlerModule.RegisterRoutes(r)
	stops = append(stops, func(ctx context.Context) {
		if err := crawlerModule.Stop(ctx); err != nil {
			deps.Log.WarnContext(ctx).Err(err).Msg("routes: crawler module stop failed")
		}
	})

	trendModule := trending.NewModule(trending.Deps{
		Mongo:             deps.Mongo,
		Redis:             deps.Redis,
		Bus:               deps.Bus,
		Log:               deps.Log,
		Cfg:               deps.Cfg,
		JWTValidator:      deps.JWTValidator,
		TenantMongoClient: deps.TenantMongoClient,
	})
	trendModule.Setup()
	trendModule.RegisterRoutes(r)
	stops = append(stops, func(ctx context.Context) {
		if err := trendModule.Stop(ctx); err != nil {
			deps.Log.WarnContext(ctx).Err(err).Msg("routes: trending module stop failed")
		}
	})

	analyticsModule := analytics.NewModule(analytics.Deps{
		Mongo:             deps.Mongo,
		Redis:             deps.Redis,
		Log:               deps.Log,
		Cfg:               deps.Cfg,
		JWTValidator:      deps.JWTValidator,
		TenantMongoClient: deps.TenantMongoClient,
	})
	analyticsModule.Setup()
	analyticsModule.RegisterRoutes(r)
	stops = append(stops, func(ctx context.Context) {
		if err := analyticsModule.Stop(ctx); err != nil {
			deps.Log.WarnContext(ctx).Err(err).Msg("routes: analytics module stop failed")
		}
	})

	crawlerAdapter := bot.NewCrawlerAdapter(crawlerModule.Service())
	trendAdapter := bot.NewTrendingAdapter(trendModule.Service())
	botModule := bot.NewModule(bot.Deps{
		Mongo:             deps.Mongo,
		Redis:             deps.Redis,
		Bus:               deps.Bus,
		Log:               deps.Log,
		Cfg:               deps.Cfg,
		TenantMongoClient: deps.TenantMongoClient,
	})
	botModule.InjectAdapters(crawlerAdapter, trendAdapter)
	botModule.Setup()
	botModule.RegisterRoutes(r)
	stops = append(stops, func(ctx context.Context) {
		if err := botModule.Stop(ctx); err != nil {
			deps.Log.WarnContext(ctx).Err(err).Msg("routes: bot module stop failed")
		}
	})

	pagesModule := pages.NewModule(pages.Deps{
		Mongo:             deps.Mongo,
		GORMClient:        deps.GORMClient,
		Redis:             deps.Redis,
		Log:               deps.Log,
		Cfg:               deps.Cfg,
		TenantMongoClient: deps.TenantMongoClient,
	})
	pagesModule.Setup()
	pagesModule.RegisterRoutes(r)
	stops = append(stops, func(ctx context.Context) {
		if err := pagesModule.Stop(ctx); err != nil {
			deps.Log.WarnContext(ctx).Err(err).Msg("routes: pages module stop failed")
		}
	})

	menusModule := menus.NewModule(menus.Deps{
		Mongo:             deps.Mongo,
		Redis:             deps.Redis,
		Log:               deps.Log,
		Cfg:               deps.Cfg,
		TenantMongoClient: deps.TenantMongoClient,
	})
	menusModule.Setup()
	menusModule.RegisterRoutes(r)
	stops = append(stops, func(ctx context.Context) {
		if err := menusModule.Stop(ctx); err != nil {
			deps.Log.WarnContext(ctx).Err(err).Msg("routes: menus module stop failed")
		}
	})

	docsModule := documents.NewModule(documents.Deps{
		Mongo:             deps.Mongo,
		Redis:             deps.Redis,
		Log:               deps.Log,
		Cfg:               deps.Cfg,
		JWTValidator:      deps.JWTValidator,
		TenantMongoClient: deps.TenantMongoClient,
		R2:                deps.R2,
		GDrive:            deps.GDrive,
	})
	docsModule.Setup()
	docsModule.RegisterRoutes(r)
	stops = append(stops, func(ctx context.Context) {
		if err := docsModule.Stop(ctx); err != nil {
			deps.Log.WarnContext(ctx).Err(err).Msg("routes: documents module stop failed")
		}
	})

	disclosureModule := public_disclosure.NewModule(public_disclosure.Deps{
		Mongo:             deps.Mongo,
		Log:               deps.Log,
		Cfg:               deps.Cfg,
		TenantMongoClient: deps.TenantMongoClient,
		DocSvc:            docsModule.Service(),
		JWTValidator:      deps.JWTValidator,
	})
	disclosureModule.Setup()
	disclosureModule.RegisterRoutes(r)
	stops = append(stops, func(ctx context.Context) {
		if err := disclosureModule.Stop(ctx); err != nil {
			deps.Log.WarnContext(ctx).Err(err).Msg("routes: public_disclosure module stop failed")
		}
	})

	recruitmentModule := recruitment.NewModule(recruitment.Deps{
		Mongo:             deps.Mongo,
		Redis:             deps.Redis,
		Log:               deps.Log,
		Cfg:               deps.Cfg,
		JWTValidator:      deps.JWTValidator,
		TenantMongoClient: deps.TenantMongoClient,
		R2:                deps.R2,
	})
	recruitmentModule.Setup()
	recruitmentModule.RegisterRoutes(r)
	stops = append(stops, func(ctx context.Context) {
		if err := recruitmentModule.Stop(ctx); err != nil {
			deps.Log.WarnContext(ctx).Err(err).Msg("routes: recruitment module stop failed")
		}
	})

	coursesModule := courses.NewModule(courses.Deps{
		Mongo:             deps.Mongo,
		Redis:             deps.Redis,
		Log:               deps.Log,
		Cfg:               deps.Cfg,
		JWTValidator:      deps.JWTValidator,
		TenantMongoClient: deps.TenantMongoClient,
	})
	coursesModule.Setup()
	coursesModule.RegisterRoutes(r)
	stops = append(stops, func(ctx context.Context) {
		if err := coursesModule.Stop(ctx); err != nil {
			deps.Log.WarnContext(ctx).Err(err).Msg("routes: courses module stop failed")
		}
	})

	elearningModule := elearning.NewModule(elearning.Deps{
		Mongo:             deps.Mongo,
		Redis:             deps.Redis,
		Log:               deps.Log,
		Cfg:               deps.Cfg,
		JWTValidator:      deps.JWTValidator,
		TenantMongoClient: deps.TenantMongoClient,
	})
	elearningModule.Setup()
	elearningModule.RegisterRoutes(r)
	stops = append(stops, func(ctx context.Context) {
		if err := elearningModule.Stop(ctx); err != nil {
			deps.Log.WarnContext(ctx).Err(err).Msg("routes: elearning module stop failed")
		}
	})

	lmsModule := lms.NewModule(lms.Deps{
		Mongo:        deps.Mongo,
		Log:          deps.Log,
		Cfg:          deps.Cfg,
		JWTValidator: deps.JWTValidator,
	})
	lmsModule.Setup()
	lmsModule.RegisterRoutes(r)
	stops = append(stops, func(ctx context.Context) {
		if err := lmsModule.Stop(ctx); err != nil {
			deps.Log.WarnContext(ctx).Err(err).Msg("routes: lms module stop failed")
		}
	})

	usersModule := users.NewModule(users.Deps{
		Mongo:             deps.Mongo,
		GORMClient:        deps.GORMClient,
		Redis:             deps.Redis,
		Log:               deps.Log,
		Cfg:               deps.Cfg,
		JWTValidator:      deps.JWTValidator,
		TenantMongoClient: deps.TenantMongoClient,
		R2:                deps.R2,
	})
	usersModule.Setup()
	usersModule.RegisterRoutes(r)
	stops = append(stops, func(ctx context.Context) {
		if err := usersModule.Stop(ctx); err != nil {
			deps.Log.WarnContext(ctx).Err(err).Msg("routes: users module stop failed")
		}
	})

	postsModule := posts.NewModule(posts.Deps{
		Mongo:             deps.Mongo,
		GORMClient:        deps.GORMClient,
		Redis:             deps.Redis,
		Log:               deps.Log,
		Cfg:               deps.Cfg,
		JWTValidator:      deps.JWTValidator,
		TenantMongoClient: deps.TenantMongoClient,
		R2:                deps.R2,
	})
	postsModule.Setup()
	postsModule.RegisterRoutes(r)
	stops = append(stops, func(ctx context.Context) {
		if err := postsModule.Stop(ctx); err != nil {
			deps.Log.WarnContext(ctx).Err(err).Msg("routes: posts module stop failed")
		}
	})

	auditModule := audit.NewModule(audit.Deps{
		Mongo:             deps.Mongo,
		Redis:             deps.Redis,
		Log:               deps.Log,
		Cfg:               deps.Cfg,
		JWTValidator:      deps.JWTValidator,
		TenantMongoClient: deps.TenantMongoClient,
	})
	auditModule.Setup()
	auditModule.RegisterRoutes(r)
	stops = append(stops, func(ctx context.Context) {
		if err := auditModule.Stop(ctx); err != nil {
			deps.Log.WarnContext(ctx).Err(err).Msg("routes: audit module stop failed")
		}
	})

	profilesModule := profiles.NewModule(profiles.Deps{
		Mongo:             deps.Mongo,
		GORMClient:        deps.GORMClient,
		Redis:             deps.Redis,
		Log:               deps.Log,
		Cfg:               deps.Cfg,
		JWTValidator:      deps.JWTValidator,
		TenantMongoClient: deps.TenantMongoClient,
	})
	profilesModule.Setup()
	profilesModule.RegisterRoutes(r)
	stops = append(stops, func(ctx context.Context) {
		if err := profilesModule.Stop(ctx); err != nil {
			deps.Log.WarnContext(ctx).Err(err).Msg("routes: profiles module stop failed")
		}
	})

	seoModule := seo.NewModule(seo.Deps{
		Mongo:             deps.Mongo,
		Redis:             deps.Redis,
		Bus:               deps.Bus,
		Log:               deps.Log,
		Cfg:               deps.Cfg,
		JWTValidator:      deps.JWTValidator,
		TenantMongoClient: deps.TenantMongoClient,
		AI:                aiModule.Service(),
	})
	seoModule.Setup()
	seoModule.RegisterRoutes(r)
	stops = append(stops, func(ctx context.Context) {
		if err := seoModule.Stop(ctx); err != nil {
			deps.Log.WarnContext(ctx).Err(err).Msg("routes: seo module stop failed")
		}
	})

	reviewsModule := reviews.NewModule(reviews.Deps{
		Mongo:             deps.Mongo,
		Redis:             deps.Redis,
		Log:               deps.Log,
		Cfg:               deps.Cfg,
		JWTValidator:      deps.JWTValidator,
		TenantMongoClient: deps.TenantMongoClient,
	})
	reviewsModule.Setup()
	reviewsModule.RegisterRoutes(r)
	stops = append(stops, func(ctx context.Context) {
		if err := reviewsModule.Stop(ctx); err != nil {
			deps.Log.WarnContext(ctx).Err(err).Msg("routes: reviews module stop failed")
		}
	})

	sessionsModule := sessions.NewModule(sessions.Deps{
		Mongo:             deps.Mongo,
		GORMClient:        deps.GORMClient,
		Redis:             deps.Redis,
		Log:               deps.Log,
		Cfg:               deps.Cfg,
		JWTValidator:      deps.JWTValidator,
		TenantMongoClient: deps.TenantMongoClient,
		AC:                accessControlModule.Service(),
	})
	sessionsModule.Setup()
	sessionsModule.RegisterRoutes(r)
	stops = append(stops, func(ctx context.Context) {
		if err := sessionsModule.Stop(ctx); err != nil {
			deps.Log.WarnContext(ctx).Err(err).Msg("routes: sessions module stop failed")
		}
	})

	sitemapModule := sitemap.NewModule(sitemap.Deps{
		Mongo:             deps.Mongo,
		GORMClient:        deps.GORMClient,
		Log:               deps.Log,
		Cfg:               deps.Cfg,
		TenantMongoClient: deps.TenantMongoClient,
	})
	sitemapModule.Setup()
	sitemapModule.RegisterRoutes(r)
	stops = append(stops, func(ctx context.Context) {
		if err := sitemapModule.Stop(ctx); err != nil {
			deps.Log.WarnContext(ctx).Err(err).Msg("routes: sitemap module stop failed")
		}
	})

	opsModule.RegisterRoutes(r)
	stops = append(stops, func(ctx context.Context) {
		if err := opsModule.Stop(ctx); err != nil {
			deps.Log.WarnContext(ctx).Err(err).Msg("routes: operations module stop failed")
		}
	})

	// Start Queue Server if configured
	if deps.QueueServer != nil {
		workerHandlers := make(map[string]func(context.Context, *asynq.Task) error)
		if aiModule != nil && aiModule.Service() != nil {
			workerHandlers[ai_content.TaskGeneratePost] = aiModule.Service().HandleGeneratePost
		}

		go func() {
			err := deps.QueueServer.Start(context.Background(), workerHandlers)
			if err != nil {
				deps.Log.Error().Err(err).Msg("routes: failed to start asynq queue server")
			}
		}()
	}

	return stops
}
