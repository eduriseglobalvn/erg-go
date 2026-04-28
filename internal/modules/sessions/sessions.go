// Package sessions implements the Sessions module.
// Pattern: NewModule → Setup → RegisterRoutes → Stop.
package sessions

import (
	"context"

	"github.com/gin-gonic/gin"

	ac "erg.ninja/internal/modules/access_control/service"
	"erg.ninja/internal/modules/sessions/controller"
	"erg.ninja/internal/modules/sessions/repository"
	"erg.ninja/internal/modules/sessions/service"
	"erg.ninja/pkg/auth"
	"erg.ninja/pkg/cache"
	"erg.ninja/pkg/config"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/tenant"
)

// Deps holds the sessions module's dependencies.
type Deps struct {
	Mongo             *database.MongoClient
	GORMClient        *database.GORMPostgresClient
	Redis             *cache.RedisClient
	Log               *logger.Logger
	Cfg               *config.Config
	JWTValidator      *auth.JWTValidator
	TenantMongoClient *tenant.TenantMongoClient
	AC                *ac.Service
}

// Module implements the erg-go module pattern.
type Module struct {
	deps Deps
	repo *repository.Repository
	svc  *service.Service
	ctrl *controller.Controller
}

// NewModule creates a new sessions module.
func NewModule(deps Deps) *Module {
	return &Module{deps: deps}
}

// Name implements plugin.Module.
func (m *Module) Name() string { return "sessions" }

// Setup initialises the module (like NestJS onModuleInit).
func (m *Module) Setup() error {
	m.deps.Log.Info().Msg("sessions: module setup")
	m.repo = repository.NewRepository(m.deps.GORMClient)
	m.svc = service.NewService(
		service.Deps{
			Repo:  m.repo,
			Redis: m.deps.Redis,
			Log:   m.deps.Log,
			AC:    m.deps.AC,
		},
	)
	m.ctrl = controller.NewController(m.svc, m.deps.JWTValidator, m.deps.Log)
	return nil
}

// RegisterRoutes mounts the sessions HTTP routes.
func (m *Module) RegisterRoutes(r *gin.Engine) {
	if m.ctrl != nil {
		m.ctrl.RegisterRoutes(r)
	}
}

// Stop performs graceful shutdown.
func (m *Module) Stop(ctx context.Context) error {
	m.deps.Log.Info().Msg("sessions: module stopped")
	return nil
}
