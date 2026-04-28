package ai_content

import (
	"context"

	"github.com/gin-gonic/gin"

	"erg.ninja/pkg/cache"
	"erg.ninja/pkg/config"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/queue"
	"erg.ninja/pkg/tenant"
)

// Deps holds the ai_content module's dependencies.
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
	repo *Repository
	q    *queue.AsynqClient
	svc  *Service
	ctrl *Controller
}

// NewModule creates a new ai_content module.
func NewModule(deps Deps) *Module { return &Module{deps: deps} }

// Name implements plugin.Module.
func (m *Module) Name() string { return "ai_content" }

// Service returns the underlying service instance.
func (m *Module) Service() *Service { return m.svc }

// Setup initialises the module.
func (m *Module) Setup() error {
	m.deps.Log.Info().Msg("ai_content: module setup")
	
	qc, err := queue.NewAsynqClient(m.deps.Cfg.Queue, queue.WithAsynqClientLogger(m.deps.Log))
	if err != nil {
		return err
	}
	m.q = qc
	m.repo = NewRepository(m.deps.Mongo)
	m.svc = NewService(m.repo, m.q, m.deps.Log)

	m.ctrl = NewController(m.svc, m.deps.Log)
	return nil
}

// RegisterRoutes mounts the ai_content HTTP routes.
func (m *Module) RegisterRoutes(r *gin.Engine) {
	if m.ctrl != nil {
		m.ctrl.RegisterRoutes(r)
	}
}

// Stop performs graceful shutdown.
func (m *Module) Stop(ctx context.Context) error {
	m.deps.Log.Info().Msg("ai_content: module stopped")
	if m.q != nil {
		m.q.Close()
	}
	return nil
}
