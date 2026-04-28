// Package elearning implements the E-learning module.
// Mirrors the NestJS module pattern: NewModule -> Setup -> RegisterRoutes -> Stop.
package elearning

import (
	"context"

	"github.com/gin-gonic/gin"

	"erg.ninja/internal/middleware"
	"erg.ninja/pkg/auth"
	"erg.ninja/pkg/cache"
	"erg.ninja/pkg/config"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/tenant"
)

// Deps holds the elearning module dependencies.
type Deps struct {
	Mongo             *database.MongoClient
	Redis             *cache.RedisClient
	Log               *logger.Logger
	Cfg               *config.Config
	JWTValidator      *auth.JWTValidator
	TenantMongoClient *tenant.TenantMongoClient
}

// Module is the elearning module entry point.
type Module struct {
	deps Deps
	repo *Repository
	svc  *Service
	ctrl *Controller
}

// NewModule creates a new elearning module.
func NewModule(deps Deps) *Module {
	return &Module{deps: deps}
}

// Name implements plugin.Module.
func (m *Module) Name() string { return "elearning" }

// Setup initialises the module.
func (m *Module) Setup() error {
	if m.deps.Log != nil {
		m.deps.Log.Info().Msg("elearning: module setup")
	}
	m.repo = NewRepository(m.deps.Mongo, WithRepositoryLogger(m.deps.Log))
	m.svc = NewService(m.repo, m.deps.Log)

	tenantID := "default"
	if m.deps.Cfg != nil && m.deps.Cfg.Tenant.DefaultID != "" {
		tenantID = m.deps.Cfg.Tenant.DefaultID
	}
	if err := m.svc.SeedElearningData(context.Background(), tenantID); err != nil {
		if m.deps.Log != nil {
			m.deps.Log.Warn().Err(err).Msg("elearning: failed to seed default data")
		}
	}

	m.ctrl = NewController(m.svc, m.deps.Log)
	return nil
}

// RegisterRoutes mounts public and admin elearning routes.
func (m *Module) RegisterRoutes(r *gin.Engine) {
	if m.ctrl == nil {
		return
	}

	m.ctrl.RegisterPublicRoutes(r)

	admin := r.Group("/")
	if m.deps.JWTValidator != nil {
		admin.Use(middleware.JWTMiddleware(m.deps.JWTValidator))
		admin.Use(middleware.RequireRoles("admin"))
	}
	m.ctrl.RegisterAdminRoutes(admin)
}

// Stop performs graceful shutdown.
func (m *Module) Stop(ctx context.Context) error {
	if m.deps.Log != nil {
		m.deps.Log.Info().Msg("elearning: module stopped")
	}
	return nil
}
