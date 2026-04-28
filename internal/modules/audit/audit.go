// Package audit implements the Audit module.
// Pattern: NewModule → Setup → RegisterRoutes → Stop.
package audit

import (
	"context"

	"github.com/gin-gonic/gin"

	auditcontroller "erg.ninja/internal/modules/audit/controller"
	auditservice "erg.ninja/internal/modules/audit/service"
	"erg.ninja/pkg/auth"
	"erg.ninja/pkg/cache"
	"erg.ninja/pkg/config"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/tenant"
)

// Deps holds the audit module's dependencies.
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
	svc  *auditservice.Service
	ctrl *auditcontroller.Controller
}

// NewModule creates a new audit module.
func NewModule(deps Deps) *Module {
	return &Module{deps: deps}
}

// Name implements plugin.Module.
func (m *Module) Name() string { return "audit" }

// Setup initialises the module.
func (m *Module) Setup() error {
	m.deps.Log.Info().Msg("audit: module setup")
	m.svc = auditservice.NewService(m.deps.Mongo, m.deps.Log)
	m.ctrl = auditcontroller.NewController(m.svc, m.deps.JWTValidator, m.deps.Log)
	return nil
}

// RegisterRoutes mounts the audit HTTP routes.
func (m *Module) RegisterRoutes(r *gin.Engine) {
	if m.ctrl != nil {
		m.ctrl.RegisterRoutes(r)
	}
}

// Stop performs graceful shutdown.
func (m *Module) Stop(ctx context.Context) error {
	m.deps.Log.Info().Msg("audit: module stopped")
	return nil
}

// Service returns the underlying service (for use by other modules for logging).
func (m *Module) Service() *auditservice.Service {
	return m.svc
}
