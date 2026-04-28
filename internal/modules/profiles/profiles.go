// Package profiles implements the Profiles module.
// Pattern: NewModule → Setup → RegisterRoutes → Stop.
package profiles

import (
	"context"

	"github.com/gin-gonic/gin"

	profilescontroller "erg.ninja/internal/modules/profiles/controller"
	profilesservice "erg.ninja/internal/modules/profiles/service"
	"erg.ninja/pkg/auth"
	"erg.ninja/pkg/cache"
	"erg.ninja/pkg/config"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/tenant"
)

// Deps holds the profiles module's dependencies.
type Deps struct {
	Mongo             *database.MongoClient
	GORMClient        *database.GORMPostgresClient
	Redis             *cache.RedisClient
	Log               *logger.Logger
	Cfg               *config.Config
	JWTValidator      *auth.JWTValidator
	TenantMongoClient *tenant.TenantMongoClient
}

// Module implements the erg-go module pattern.
type Module struct {
	deps Deps
	svc  *profilesservice.Service
	ctrl *profilescontroller.Controller
}

// NewModule creates a new profiles module.
func NewModule(deps Deps) *Module {
	return &Module{deps: deps}
}

// Name implements plugin.Module.
func (m *Module) Name() string { return "profiles" }

// Setup initialises the module.
func (m *Module) Setup() error {
	m.deps.Log.Info().Msg("profiles: module setup")
	m.svc = profilesservice.NewService(m.deps.Mongo, m.deps.GORMClient, m.deps.Log)
	if err := m.svc.BackfillLegacyProfiles(context.Background(), m.deps.Mongo); err != nil {
		m.deps.Log.Warn().Err(err).Msg("profiles: legacy profile backfill failed")
	}
	m.ctrl = profilescontroller.NewController(m.svc, m.deps.JWTValidator, m.deps.Log)
	return nil
}

// RegisterRoutes mounts the profiles HTTP routes.
func (m *Module) RegisterRoutes(r *gin.Engine) {
	if m.ctrl != nil {
		m.ctrl.RegisterRoutes(r)
	}
}

// Stop performs graceful shutdown.
func (m *Module) Stop(ctx context.Context) error {
	m.deps.Log.Info().Msg("profiles: module stopped")
	return nil
}

// Service returns the underlying service (for use by other modules).
func (m *Module) Service() *profilesservice.Service {
	return m.svc
}
