// Package posts implements the Posts module.
// Pattern: NewModule → Setup → RegisterRoutes → Stop.
package posts

import (
	"context"

	"github.com/gin-gonic/gin"

	"erg.ninja/internal/modules/posts/api/controller"
	"erg.ninja/internal/modules/posts/application/service"
	"erg.ninja/internal/modules/posts/infrastructure/repository"
	"erg.ninja/pkg/auth"
	"erg.ninja/pkg/cache"
	"erg.ninja/pkg/config"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/storage"
	"erg.ninja/pkg/tenant"
)

// Deps holds the posts module's dependencies.
type Deps struct {
	Mongo             *database.MongoClient
	GORMClient        *database.GORMPostgresClient
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
	svc  *service.Service
	ctrl *controller.Controller
}

// NewModule creates a new posts module.
func NewModule(deps Deps) *Module {
	return &Module{deps: deps}
}

// Name implements plugin.Module.
func (m *Module) Name() string { return "posts" }

// Setup initialises the module.
func (m *Module) Setup() error {
	m.deps.Log.Info().Msg("posts: module setup")
	repo := repository.NewRepository(m.deps.GORMClient, m.deps.Log)
	m.svc = service.NewService(repo, m.deps.Redis, m.deps.Log, m.deps.R2)

	if err := m.svc.SeedDefaultCategories(context.Background()); err != nil {
		m.deps.Log.Warn().Err(err).Msg("posts: failed to seed default categories")
	}

	m.ctrl = controller.NewController(m.svc, m.deps.JWTValidator, m.deps.Log)
	return nil
}

// RegisterRoutes mounts the posts HTTP routes.
func (m *Module) RegisterRoutes(r *gin.Engine) {
	if m.ctrl != nil {
		m.ctrl.RegisterRoutes(r)
	}
}

// Stop performs graceful shutdown.
func (m *Module) Stop(ctx context.Context) error {
	m.deps.Log.Info().Msg("posts: module stopped")
	return nil
}
