package auth

import (
	"context"

	"github.com/gin-gonic/gin"

	ac "erg.ninja/internal/modules/access_control/application/service"
	"erg.ninja/internal/modules/auth/api/controller"
	"erg.ninja/internal/modules/auth/application/service"
	"erg.ninja/internal/modules/auth/infrastructure/repository"
	"erg.ninja/pkg/auth"
	"erg.ninja/pkg/cache"
	"erg.ninja/pkg/config"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/storage"
	"erg.ninja/pkg/tenant"
)

// Deps holds the auth module's dependencies.
type Deps struct {
	Mongo             *database.MongoClient
	GORMClient        *database.GORMPostgresClient
	Redis             *cache.RedisClient
	Log               *logger.Logger
	Cfg               *config.Config
	JWTValidator      *auth.JWTValidator
	TenantMongoClient *tenant.TenantMongoClient
	AC                *ac.Service
	R2                *storage.R2Client
}

// Module is the auth module.
type Module struct {
	deps Deps
	repo *repository.Repo
	svc  *service.AuthService
	ctrl *controller.Controller
}

// NewModule creates a new auth module.
func NewModule(deps Deps) *Module {
	return &Module{deps: deps}
}

// Name implements plugin.Module.
func (m *Module) Name() string { return "auth" }

// Setup implements plugin.Module.
func (m *Module) Setup() error {
	m.deps.Log.Info().Msg("auth: module setup")
	m.repo = repository.NewRepo(repository.RepoDeps{
		GORM:  m.deps.GORMClient,
		Redis: m.deps.Redis,
	})
	m.svc = service.NewAuthService(service.ServiceDeps{
		Repo:   m.repo,
		Redis:  m.deps.Redis,
		Log:    m.deps.Log,
		Config: m.deps.Cfg,
		AC:     m.deps.AC,
		R2:     m.deps.R2,
	})
	m.ctrl = controller.NewController(m.svc, m.deps.JWTValidator, m.deps.Log, m.deps.Cfg)

	if m.deps.Cfg != nil && m.deps.Cfg.Lifecycle.AuthBootstrapAdminOnStartup {
		if err := m.svc.BootstrapAdmin(); err != nil {
			m.deps.Log.Error().Err(err).Msg("auth: BootstrapAdmin failed — continuing startup")
		} else {
			m.deps.Log.Info().Msg("auth: module setup complete (admin bootstrap done)")
		}
	}

	return nil
}

// RegisterRoutes mounts the auth module's HTTP routes onto the Gin router.
func (m *Module) RegisterRoutes(r *gin.Engine) {
	m.ctrl.RegisterRoutes(r)
}

// Stop implements plugin.Module.
func (m *Module) Stop(ctx context.Context) error {
	if m.deps.Log != nil {
		m.deps.Log.Info().Msg("auth: module stopped")
	}
	return nil
}
