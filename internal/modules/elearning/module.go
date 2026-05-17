// Package elearning implements the E-learning module.
// Mirrors the NestJS module pattern: NewModule -> Setup -> RegisterRoutes -> Stop.
package elearning

import (
	"context"

	"github.com/gin-gonic/gin"

	"erg.ninja/internal/middleware"
	elearningcontroller "erg.ninja/internal/modules/elearning/api/controller"
	elearningservice "erg.ninja/internal/modules/elearning/application/service"
	elearningrepo "erg.ninja/internal/modules/elearning/infrastructure/repository"
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
	repo *elearningrepo.Repository
	svc  *elearningservice.Service
	ctrl *elearningcontroller.Controller
}

type Service = elearningservice.Service
type StudentActor = elearningservice.StudentActor

// NewModule creates a new elearning module.
func NewModule(deps Deps) *Module {
	return &Module{deps: deps}
}

func NewController(svc *Service, log *logger.Logger) *elearningcontroller.Controller {
	return elearningcontroller.NewController(svc, log)
}

func NewService(repo *elearningrepo.Repository, log *logger.Logger) *elearningservice.Service {
	return elearningservice.NewService(repo, log)
}

// Name implements plugin.Module.
func (m *Module) Name() string { return "elearning" }

// Setup initialises the module.
func (m *Module) Setup() error {
	if m.deps.Log != nil {
		m.deps.Log.Info().Msg("elearning: module setup")
	}
	m.repo = elearningrepo.NewRepository(m.deps.Mongo, elearningrepo.WithRepositoryLogger(m.deps.Log))
	m.svc = elearningservice.NewService(m.repo, m.deps.Log)

	tenantID := "default"
	if m.deps.Cfg != nil && m.deps.Cfg.Tenant.DefaultID != "" {
		tenantID = m.deps.Cfg.Tenant.DefaultID
	}
	if err := m.svc.SeedElearningData(context.Background(), tenantID); err != nil {
		if m.deps.Log != nil {
			m.deps.Log.Warn().Err(err).Msg("elearning: failed to seed default data")
		}
	}

	m.ctrl = elearningcontroller.NewController(m.svc, m.deps.Log)
	return nil
}

// RegisterRoutes mounts public and admin elearning routes.
func (m *Module) RegisterRoutes(r *gin.Engine) {
	if m.ctrl == nil {
		return
	}

	m.ctrl.RegisterPublicRoutes(r)

	student := r.Group("/api/elearning")
	student.Use(middleware.JWTMiddleware(m.deps.JWTValidator))
	student.Use(middleware.RequirePortal("elearning"))
	m.ctrl.RegisterStudentRoutes(student)

	admin := r.Group("/")
	admin.Use(middleware.JWTMiddleware(m.deps.JWTValidator))
	admin.Use(middleware.RequirePortal("elearning"))
	admin.Use(middleware.RequireAccessPermission("elearning.*"))
	m.ctrl.RegisterAdminRoutes(admin)
}

// Stop performs graceful shutdown.
func (m *Module) Stop(ctx context.Context) error {
	if m.deps.Log != nil {
		m.deps.Log.Info().Msg("elearning: module stopped")
	}
	return nil
}
