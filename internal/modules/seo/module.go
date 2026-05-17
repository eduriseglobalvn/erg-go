package seo

import (
	"context"

	"github.com/gin-gonic/gin"

	seocontroller "erg.ninja/internal/modules/seo/api/controller"
	seoservice "erg.ninja/internal/modules/seo/application/service"
	seorepo "erg.ninja/internal/modules/seo/infrastructure/repository"
	auth "erg.ninja/pkg/auth"
	"erg.ninja/pkg/cache"
	"erg.ninja/pkg/config"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/event"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/tenant"
)

// Deps holds the SEO module's dependencies.
type Deps struct {
	Mongo             *database.MongoClient
	Redis             *cache.RedisClient
	Bus               *event.EventBus
	Log               *logger.Logger
	Cfg               *config.Config
	JWTValidator      *auth.JWTValidator
	TenantMongoClient *tenant.TenantMongoClient
	AI                AIService
}

// Module is the SEO module. It implements the module pattern used across all modules.
type Module struct {
	deps   Deps
	repo   *seorepo.Repository
	svc    *seoservice.Service
	ctrl   *seocontroller.Controller
	jwtVal *auth.JWTValidator
}

type AIService = seoservice.AIService
type Service = seoservice.Service

// NewModule creates a new SEO module with the given dependencies.
func NewModule(deps Deps) *Module {
	return &Module{deps: deps}
}

// Name implements plugin.Module.
func (m *Module) Name() string { return "seo" }

// Setup implements plugin.Module.
func (m *Module) Setup() error {
	m.deps.Log.Info().Msg("seo: module setup")
	m.repo = seorepo.NewRepository(m.deps.Mongo, seorepo.WithRepositoryLogger(m.deps.Log))
	m.svc = seoservice.NewService(m.repo, m.deps.AI, m.deps.Log)
	m.ctrl = seocontroller.NewController(m.svc, m.deps.Log)
	m.jwtVal = m.deps.JWTValidator
	return nil
}

// RegisterRoutes mounts the SEO module's HTTP routes onto the Gin router.
func (m *Module) RegisterRoutes(r *gin.Engine) {
	m.ctrl.RegisterRoutes(r, m.jwtVal)
}

// Stop implements plugin.Module — performs graceful shutdown.
func (m *Module) Stop(ctx context.Context) error {
	if m.deps.Log != nil {
		m.deps.Log.Info().Msg("seo: module stopped")
	}
	return nil
}

// Service returns the SEO service instance (for cross-module integration).
func (m *Module) Service() *seoservice.Service { return m.svc }
