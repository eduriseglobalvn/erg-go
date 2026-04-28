package trending

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"

	"erg.ninja/internal/middleware"
	trendingcache "erg.ninja/internal/modules/trending/cache"
	auth "erg.ninja/pkg/auth"
	"erg.ninja/pkg/cache"
	"erg.ninja/pkg/config"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/event"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/tenant"
)

// Deps holds the trending module's dependencies.
type Deps struct {
	Mongo             *database.MongoClient
	Redis             *cache.RedisClient
	Bus               *event.EventBus
	Log               *logger.Logger
	Cfg               *config.Config
	JWTValidator      *auth.JWTValidator
	TenantMongoClient *tenant.TenantMongoClient
}

type Module struct {
	deps      Deps
	repo      *Repository
	cache     *trendingcache.RedisCache
	svc       *Service
	ctrl      *Controller
	scheduler *Scheduler
	jwtVal    *auth.JWTValidator
}

func NewModule(deps Deps) *Module {
	return &Module{deps: deps}
}

// Name implements plugin.Module.
func (m *Module) Name() string { return "trending" }

// Setup implements plugin.Module.
func (m *Module) Setup() error {
	m.repo = NewRepository(m.deps.Mongo, WithRepositoryLogger(m.deps.Log))
	m.cache = trendingcache.NewRedisCache(m.deps.Redis, m.deps.Cfg.Trending.CacheTTL)
	m.svc = NewService(m.repo, m.cache, m.deps.Log, m.deps.Bus, m.deps.Cfg.Trending)
	m.jwtVal = m.deps.JWTValidator
	m.ctrl = NewController(m.svc, m.deps.Mongo, m.deps.Redis, m.deps.Log, m.jwtVal)
	m.scheduler = NewScheduler(m.svc, m.deps.Log, m.deps.Cfg.Trending.RefreshCron)
	if err := m.scheduler.Start(); err != nil {
		m.deps.Log.Error().Err(err).Msg("trending: scheduler start failed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := m.svc.Refresh(ctx); err != nil {
		m.deps.Log.Warn().Err(err).Msg("trending: initial refresh failed")
	}
	m.deps.Log.Info().Msg("trending: module setup")
	return nil
}

func (m *Module) RegisterRoutes(r *gin.Engine) {
	// Public healthz/ready endpoints — registered directly, no auth.
	m.ctrl.RegisterPublicRoutes(r)

	// Protected /api/trending/* — JWT guard applied via gin.RouterGroup.
	protected := r.Group("/api/trending")
	if m.jwtVal != nil {
		protected.Use(middleware.JWTMiddleware(m.jwtVal))
	} else {
		m.deps.Log.Warn().Msg("trending: JWT validator not configured — endpoints are UNPROTECTED")
	}
	m.ctrl.RegisterProtectedRoutes(protected)
}

// Stop implements plugin.Module — performs graceful shutdown.
func (m *Module) Stop(ctx context.Context) error {
	if m.scheduler != nil {
		stopCtx := m.scheduler.Stop()
		select {
		case <-stopCtx.Done():
		case <-ctx.Done():
		}
	}
	if m.deps.Log != nil {
		m.deps.Log.Info().Msg("trending: module stopped")
	}
	return nil
}

// Service returns the trending service instance (for cross-module integration).
func (m *Module) Service() *Service { return m.svc }
