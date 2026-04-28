package public_disclosure

import (
	"context"

	"github.com/gin-gonic/gin"

	"erg.ninja/internal/modules/documents"
	"erg.ninja/internal/modules/public_disclosure/repository"
	"erg.ninja/internal/modules/public_disclosure/service"
	"erg.ninja/pkg/config"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/tenant"
)

type Deps struct {
	Mongo             *database.MongoClient
	Log               *logger.Logger
	Cfg               *config.Config
	TenantMongoClient *tenant.TenantMongoClient
	DocSvc            *documents.Service
}

type Module struct {
	deps Deps
	repo *repository.Repository
	svc  *service.Service
	ctrl *Controller
}

func NewModule(deps Deps) *Module {
	repo := repository.NewRepository(deps.Mongo)
	svc := service.NewService(repo, deps.DocSvc, deps.Log)
	ctrl := NewController(svc, deps.Log)

	return &Module{
		deps: deps,
		repo: repo,
		svc:  svc,
		ctrl: ctrl,
	}
}

func (m *Module) Setup() error {
	return nil
}

func (m *Module) RegisterRoutes(r *gin.Engine) {
	m.ctrl.RegisterRoutes(r)
}

func (m *Module) Stop(ctx context.Context) error {
	return nil
}

func (m *Module) Service() *service.Service {
	return m.svc
}
