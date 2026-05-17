package hoclieu

import (
	"context"

	"github.com/gin-gonic/gin"

	"erg.ninja/internal/middleware"
	hoclieucontroller "erg.ninja/internal/modules/hoclieu/api/controller"
	hoclieuservice "erg.ninja/internal/modules/hoclieu/application/service"
	hoclieurepository "erg.ninja/internal/modules/hoclieu/infrastructure/repository"
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
	R2           *storage.R2Client
	Mongo        *database.MongoClient
}

type Module struct {
	deps Deps
	svc  *hoclieuservice.Service
	ctrl *hoclieucontroller.Controller
}

func NewModule(deps Deps) *Module {
	return &Module{deps: deps}
}

func (m *Module) Name() string { return "hoclieu" }

func (m *Module) Setup() error {
	m.svc = hoclieuservice.NewService(m.deps.R2)
	if m.deps.Mongo != nil {
		repo := hoclieurepository.NewRepository(m.deps.Mongo)
		defaultTenantID := "default"
		if m.deps.Cfg != nil && m.deps.Cfg.Tenant.DefaultID != "" {
			defaultTenantID = m.deps.Cfg.Tenant.DefaultID
		}
		m.svc.UseRepository(repo, defaultTenantID)
		if err := m.svc.EnsurePersistentStore(context.Background()); err != nil {
			return err
		}
	}
	m.ctrl = hoclieucontroller.NewController(m.svc)
	if m.deps.Log != nil {
		m.deps.Log.Info().Msg("hoclieu: module setup")
	}
	return nil
}

func (m *Module) RegisterRoutes(r *gin.Engine) {
	base := r.Group("/api/hoclieu")
	m.ctrl.RegisterPublicRoutes(base)

	protected := base.Group("")
	protected.Use(middleware.JWTMiddleware(m.deps.JWTValidator))
	protected.Use(middleware.RequirePortal("hoclieu"))
	m.ctrl.RegisterProtectedRoutes(protected)
}

func (m *Module) Stop(ctx context.Context) error {
	if m.deps.Log != nil {
		m.deps.Log.Info().Msg("hoclieu: module stopped")
	}
	return nil
}
