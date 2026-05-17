// Package analytics implements the Analytics module.
// Pattern: NewModule → Setup → RegisterRoutes → Stop.
package analytics

import (
	"context"

	"github.com/gin-gonic/gin"

	"erg.ninja/internal/modules/analytics/api/controller"
	"erg.ninja/internal/modules/analytics/application/service"
	"erg.ninja/pkg/auth"
	"erg.ninja/pkg/cache"
	"erg.ninja/pkg/config"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/tenant"
)

// Deps holds the analytics module's dependencies.
type Deps struct {
	Mongo             *database.MongoClient
	Redis             *cache.RedisClient
	Log               *logger.Logger
	Cfg               *config.Config
	JWTValidator      *auth.JWTValidator
	TenantMongoClient *tenant.TenantMongoClient
}

// Module implements the erg-go module pattern.
type Module struct {
	deps Deps
	svc  *service.Service
	ctrl *controller.Controller
}

// NewModule creates a new analytics module.
func NewModule(deps Deps) *Module {
	return &Module{deps: deps}
}

// Name implements plugin.Module.
func (m *Module) Name() string { return "analytics" }

// Setup initialises the module.
func (m *Module) Setup() error {
	m.deps.Log.Info().Msg("analytics: module setup")
	m.svc = service.NewService(m.deps.Mongo, m.deps.Log, m.deps.Redis, m.deps.Cfg)
	m.ctrl = controller.NewController(m.svc, m.deps.Log, m.deps.Cfg, m.deps.JWTValidator)
	return nil
}

// RegisterRoutes mounts the analytics HTTP routes.
func (m *Module) RegisterRoutes(r *gin.Engine) {
	if m.ctrl != nil {
		m.ctrl.RegisterRoutes(r)
	}
}

// Stop performs graceful shutdown.
func (m *Module) Stop(ctx context.Context) error {
	m.deps.Log.Info().Msg("analytics: module stopped")
	return nil
}
