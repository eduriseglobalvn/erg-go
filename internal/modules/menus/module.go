// Package menus implements the CMS Navigation Menus module.
// Pattern: NewModule → Setup → RegisterRoutes → Stop (no database).
package menus

import (
	"context"

	"github.com/gin-gonic/gin"

	menuscontroller "erg.ninja/internal/modules/menus/api/controller"
	menusservice "erg.ninja/internal/modules/menus/application/service"
	"erg.ninja/pkg/cache"
	"erg.ninja/pkg/config"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/tenant"
)

// Deps holds the menus module's dependencies.
type Deps struct {
	Mongo             *database.MongoClient
	Redis             *cache.RedisClient
	Log               *logger.Logger
	Cfg               *config.Config
	TenantMongoClient *tenant.TenantMongoClient
}

// Module implements the erg-go module pattern.
type Module struct {
	deps Deps
	svc  *menusservice.Service
	ctrl *menuscontroller.Controller
}

// NewModule creates a new menus module.
func NewModule(deps Deps) *Module {
	return &Module{deps: deps}
}

// Name implements plugin.Module.
func (m *Module) Name() string { return "menus" }

// Setup initialises the module.
func (m *Module) Setup() error {
	m.deps.Log.Info().Msg("menus: module setup")
	m.svc = menusservice.NewService(m.deps.Log)
	m.ctrl = menuscontroller.NewController(m.svc, m.deps.Log)
	return nil
}

// RegisterRoutes mounts the menus HTTP routes.
func (m *Module) RegisterRoutes(r *gin.Engine) {
	if m.ctrl != nil {
		m.ctrl.RegisterRoutes(r)
	}
}

// Stop performs graceful shutdown.
func (m *Module) Stop(ctx context.Context) error {
	m.deps.Log.Info().Msg("menus: module stopped")
	return nil
}
