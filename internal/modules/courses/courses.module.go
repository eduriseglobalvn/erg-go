// Package courses implements the Courses module.
// Mirrors the NestJS module pattern: NewModule → Setup → RegisterRoutes → Stop.
package courses

import (
	"context"

	"github.com/gin-gonic/gin"

	"erg.ninja/internal/middleware"
	"erg.ninja/internal/modules/courses/controller"
	"erg.ninja/internal/modules/courses/repository"
	"erg.ninja/internal/modules/courses/service"
	"erg.ninja/pkg/auth"
	"erg.ninja/pkg/cache"
	"erg.ninja/pkg/config"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/tenant"
)

// Deps holds the courses module's dependencies.
type Deps struct {
	Mongo             *database.MongoClient
	Redis             *cache.RedisClient
	Log               *logger.Logger
	Cfg               *config.Config
	JWTValidator      *auth.JWTValidator
	TenantMongoClient *tenant.TenantMongoClient
}

// Module is the courses module.
type Module struct {
	deps Deps
	repo *repository.Repository
	svc  *service.Service
	ctrl *controller.Controller
}

// NewModule creates a new courses module.
func NewModule(deps Deps) *Module {
	return &Module{deps: deps}
}

// Name implements plugin.Module.
func (m *Module) Name() string { return "courses" }

// Setup implements plugin.Module.
func (m *Module) Setup() error {
	m.deps.Log.Info().Msg("courses: module setup")
	m.repo = repository.NewRepository(m.deps.Mongo,
		repository.WithRepositoryLogger(m.deps.Log),
	)
	m.svc = service.NewService(m.repo,
		service.WithCourseLogger(m.deps.Log),
	)
	m.ctrl = controller.NewController(m.svc, m.deps.Log)
	return nil
}

// RegisterRoutes mounts the courses module's HTTP routes onto the Gin router.
func (m *Module) RegisterRoutes(r *gin.Engine) {
	base := r.Group("/api/courses")

	// Public routes
	m.ctrl.RegisterPublicRoutes(base)

	// Admin routes — auth required
	admin := base.Group("")
	admin.Use(middleware.JWTMiddleware(m.deps.JWTValidator))
	admin.Use(middleware.RequireRoles("admin"))
	m.ctrl.RegisterAdminRoutes(admin)
}

// Stop implements plugin.Module.
func (m *Module) Stop(ctx context.Context) error {
	if m.deps.Log != nil {
		m.deps.Log.Info().Msg("courses: module stopped")
	}
	return nil
}
