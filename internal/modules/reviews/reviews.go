// Package reviews implements the Reviews module.
// Pattern: NewModule → Setup → RegisterRoutes → Stop.
package reviews

import (
	"context"

	"github.com/gin-gonic/gin"

	"erg.ninja/internal/modules/reviews/controller"
	"erg.ninja/internal/modules/reviews/repository"
	"erg.ninja/internal/modules/reviews/service"
	"erg.ninja/pkg/auth"
	"erg.ninja/pkg/cache"
	"erg.ninja/pkg/config"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/tenant"
)

// Deps holds the reviews module's dependencies.
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
	repo *repository.Repository
	svc  *service.Service
	ctrl *controller.Controller
}

// NewModule creates a new reviews module.
func NewModule(deps Deps) *Module {
	return &Module{deps: deps}
}

// Name implements plugin.Module.
func (m *Module) Name() string { return "reviews" }

// Setup initialises the module.
func (m *Module) Setup() error {
	m.deps.Log.Info().Msg("reviews: module setup")
	m.repo = repository.NewRepository(m.deps.Mongo)
	m.svc = service.NewService(m.repo, m.deps.Log)
	m.ctrl = controller.NewController(m.svc, m.deps.Log, m.deps.JWTValidator)
	return nil
}

// RegisterRoutes mounts the reviews HTTP routes.
func (m *Module) RegisterRoutes(r *gin.Engine) {
	if m.ctrl != nil {
		m.ctrl.RegisterRoutes(r)
	}
}

// Stop performs graceful shutdown.
func (m *Module) Stop(ctx context.Context) error {
	m.deps.Log.Info().Msg("reviews: module stopped")
	return nil
}
