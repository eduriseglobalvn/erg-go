// Package recruitment implements the Recruitment / Tuyển dụng module.
// Mirrors the NestJS module pattern: NewModule → Setup → RegisterRoutes → Stop.
//
// Public endpoints (no auth):
//
//	GET  /api/recruitment/jobs          — list active jobs with filter/pagination
//	GET  /api/recruitment/jobs/:slug   — job detail + Schema.org JSON-LD for SEO
//	POST /api/recruitment/apply       — apply with CV upload (multipart/form-data)
//	GET  /api/recruitment/tracking/:code — track application by UUID tracking code
//
// Admin endpoints (JWT auth required):
//
//	POST                                 /api/recruitment/jobs
//	PUT                                  /api/recruitment/jobs/:id
//	DELETE                               /api/recruitment/jobs/:id
//	PATCH                                /api/recruitment/jobs/:id/toggle-hot
//	PATCH                                /api/recruitment/jobs/:id/toggle-urgent
//	PATCH                                /api/recruitment/jobs/:id/status
//	GET                                  /api/recruitment/admin/candidates
//	PATCH                                /api/recruitment/admin/candidates/:id/status
package recruitment

import (
	"context"

	"github.com/gin-gonic/gin"

	"erg.ninja/internal/modules/recruitment/api/controller"
	"erg.ninja/internal/modules/recruitment/application/service"
	"erg.ninja/internal/modules/recruitment/infrastructure/repository"
	"erg.ninja/pkg/auth"
	"erg.ninja/pkg/cache"
	"erg.ninja/pkg/config"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/storage"
	"erg.ninja/pkg/tenant"
)

// Deps holds the recruitment module's dependencies.
type Deps struct {
	Mongo             *database.MongoClient
	Redis             *cache.RedisClient
	Log               *logger.Logger
	Cfg               *config.Config
	JWTValidator      *auth.JWTValidator
	TenantMongoClient *tenant.TenantMongoClient
	R2                *storage.R2Client
}

// Module implements the erg-go module pattern.
type Module struct {
	deps Deps
	repo *repository.Repository
	svc  *service.Service
	ctrl *controller.Controller
}

// NewModule creates a new recruitment module.
func NewModule(deps Deps) *Module {
	return &Module{deps: deps}
}

// Name implements plugin.Module.
func (m *Module) Name() string { return "recruitment" }

// Setup initialises the module (like NestJS onModuleInit).
func (m *Module) Setup() error {
	m.deps.Log.Info().Msg("recruitment: module setup")
	m.repo = repository.NewRepository(m.deps.Mongo,
		repository.WithRecruitmentLogger(m.deps.Log),
	)
	m.svc = service.NewService(
		m.repo,
		m.deps.Log,
		service.WithR2(m.deps.R2),
	)
	m.ctrl = controller.NewController(m.svc, m.deps.Log, m.deps.JWTValidator)
	return nil
}

// RegisterRoutes mounts the recruitment HTTP routes.
func (m *Module) RegisterRoutes(r *gin.Engine) {
	if m.ctrl != nil {
		m.ctrl.RegisterRoutes(r)
	}
}

// Stop performs graceful shutdown.
func (m *Module) Stop(ctx context.Context) error {
	m.deps.Log.Info().Msg("recruitment: module stopped")
	return nil
}
