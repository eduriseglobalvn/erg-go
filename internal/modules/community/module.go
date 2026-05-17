package community

import (
	"context"

	"github.com/gin-gonic/gin"

	"erg.ninja/internal/middleware"
	communitycontroller "erg.ninja/internal/modules/community/api/controller"
	communityservice "erg.ninja/internal/modules/community/application/service"
	communityrepo "erg.ninja/internal/modules/community/infrastructure/repository"
	"erg.ninja/pkg/auth"
	"erg.ninja/pkg/config"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/storage"
)

type Deps struct {
	Log          *logger.Logger
	Cfg          *config.Config
	JWTValidator *auth.JWTValidator
	GORMClient   *database.GORMPostgresClient
	R2           *storage.R2Client
}

type Module struct {
	deps Deps
	repo *communityrepo.Repository
	svc  *communityservice.Service
	ctrl *communitycontroller.Controller
}

func NewModule(deps Deps) *Module {
	return &Module{deps: deps}
}

func (m *Module) Name() string { return "community" }

func (m *Module) Setup() error {
	tenantID := "default"
	if m.deps.Cfg != nil && m.deps.Cfg.Tenant.DefaultID != "" {
		tenantID = m.deps.Cfg.Tenant.DefaultID
	}
	m.repo = communityrepo.NewRepository(m.deps.GORMClient, m.deps.Log)
	m.svc = communityservice.NewService(m.repo, m.deps.Log, m.deps.R2, tenantID)
	m.ctrl = communitycontroller.NewController(m.svc)
	if err := m.svc.Setup(context.Background()); err != nil {
		if m.deps.Log != nil {
			m.deps.Log.Warn().Err(err).Msg("community: setup completed with degraded store")
		}
	}
	if m.deps.Log != nil {
		m.deps.Log.Info().Msg("community: module setup")
	}
	return nil
}

func (m *Module) RegisterRoutes(r *gin.Engine) {
	base := r.Group("/api/hoclieu/community")
	m.ctrl.RegisterPublicRoutes(base)

	authenticated := base.Group("")
	authenticated.Use(middleware.JWTMiddleware(m.deps.JWTValidator))
	m.ctrl.RegisterAuthenticatedRoutes(authenticated)
}

func (m *Module) Stop(ctx context.Context) error {
	if m.deps.Log != nil {
		m.deps.Log.Info().Msg("community: module stopped")
	}
	return nil
}
