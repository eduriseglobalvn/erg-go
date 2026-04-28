package sitemap

import (
	"context"

	"github.com/gin-gonic/gin"

	"erg.ninja/pkg/config"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/tenant"
)

// Deps holds dependencies for the sitemap module.
type Deps struct {
	Mongo             *database.MongoClient
	GORMClient        *database.GORMPostgresClient
	Log               *logger.Logger
	Cfg               *config.Config
	TenantMongoClient *tenant.TenantMongoClient
}

// Module encapsulates the sitemap functionality.
type Module struct {
	deps Deps
	ctrl *Controller
	svc  *Service
}

// NewModule creates a new sitemap module.
func NewModule(deps Deps) *Module {
	svc := NewService(deps.GORMClient, deps.Log)
	ctrl := NewController(svc, deps.Log, deps.Cfg)

	return &Module{
		deps: deps,
		ctrl: ctrl,
		svc:  svc,
	}
}

// Setup initializes the module.
func (m *Module) Setup() error {
	m.deps.Log.Info().Msg("sitemap: module initialized")
	return nil
}

// RegisterRoutes registers the sitemap API endpoints onto the router.
func (m *Module) RegisterRoutes(r *gin.Engine) {
	m.ctrl.RegisterRoutes(r)
}

// Stop gracefully shuts down the module.
func (m *Module) Stop(ctx context.Context) error {
	m.deps.Log.Info().Msg("sitemap: module stopped")
	return nil
}
