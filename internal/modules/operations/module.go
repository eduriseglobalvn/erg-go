package operations

import (
	"context"

	"github.com/gin-gonic/gin"

	operationscontroller "erg.ninja/internal/modules/operations/api/controller"
	operationsservice "erg.ninja/internal/modules/operations/application/service"
	"erg.ninja/pkg/auth"
	"erg.ninja/pkg/cache"
	"erg.ninja/pkg/config"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/tenant"
)

type Deps struct {
	Mongo             *database.MongoClient
	GORMClient        *database.GORMPostgresClient
	Redis             *cache.RedisClient
	Log               *logger.Logger
	Cfg               *config.Config
	JWTValidator      *auth.JWTValidator
	TenantMongoClient *tenant.TenantMongoClient
}

type Module struct {
	deps Deps
	svc  *operationsservice.Service
	ctrl *operationscontroller.Controller
}

func NewModule(deps Deps) *Module {
	return &Module{deps: deps}
}

func (m *Module) Name() string { return "operations" }

func (m *Module) Setup() error {
	m.deps.Log.Info().Msg("operations: module setup")
	m.svc = operationsservice.NewService(m.deps.Mongo, m.deps.GORMClient, m.deps.Redis, m.deps.Log)

	if m.deps.Cfg != nil && m.deps.Cfg.Lifecycle.OperationSeedOnStartup {
		if err := m.svc.SeedDefaults(context.Background()); err != nil {
			m.deps.Log.Warn().Err(err).Msg("operations: failed to seed defaults")
		}
	}

	m.ctrl = operationscontroller.NewController(m.svc, m.deps.Log)
	return nil
}

func (m *Module) RegisterRoutes(r *gin.Engine) {
	m.ctrl.RegisterRoutes(r, m.deps.JWTValidator)
}

func (m *Module) Stop(ctx context.Context) error {
	m.deps.Log.Info().Msg("operations: module stopped")
	return nil
}

func (m *Module) Service() *operationsservice.Service { return m.svc }
