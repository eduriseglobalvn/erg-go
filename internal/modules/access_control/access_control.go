// Package access_control implements the Access Control module.
// Pattern: NewModule → Setup → RegisterRoutes → Stop.
package access_control

import (
	"context"

	"github.com/gin-gonic/gin"

	accontroller "erg.ninja/internal/modules/access_control/controller"
	acservice "erg.ninja/internal/modules/access_control/service"
	"erg.ninja/pkg/auth"
	"erg.ninja/pkg/cache"
	"erg.ninja/pkg/config"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/tenant"
)

// Deps holds the access_control module's dependencies.
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
	svc  *acservice.Service
	ctrl *accontroller.Controller
}

// NewModule creates a new access_control module.
func NewModule(deps Deps) *Module {
	return &Module{deps: deps}
}

// Name implements plugin.Module.
func (m *Module) Name() string { return "access_control" }

// Setup initialises the module.
func (m *Module) Setup() error {
	m.deps.Log.Info().Msg("access_control: module setup")
	m.svc = acservice.NewService(m.deps.GORMClient, m.deps.Log)

	// Seed default data on startup.
	if err := m.svc.SeedDefaultData(context.Background()); err != nil {
		m.deps.Log.Warn().Err(err).Msg("access_control: seed default data failed (may already exist)")
	}

	m.ctrl = accontroller.NewController(m.svc, m.deps.JWTValidator, m.deps.Log)
	return nil
}

// RegisterRoutes mounts the access control HTTP routes.
func (m *Module) RegisterRoutes(r *gin.Engine) {
	if m.ctrl != nil {
		m.ctrl.RegisterRoutes(r)
	}
}

// Stop performs graceful shutdown.
func (m *Module) Stop(ctx context.Context) error {
	m.deps.Log.Info().Msg("access_control: module stopped")
	return nil
}

// Service returns the underlying service (for use by other modules).
func (m *Module) Service() *acservice.Service {
	return m.svc
}
