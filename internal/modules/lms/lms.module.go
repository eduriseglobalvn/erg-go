package lms

import (
	"context"

	"github.com/gin-gonic/gin"

	"erg.ninja/internal/middleware"
	"erg.ninja/pkg/auth"
	"erg.ninja/pkg/config"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/storage"
)

type Deps struct {
	Mongo        *database.MongoClient
	Log          *logger.Logger
	Cfg          *config.Config
	JWTValidator *auth.JWTValidator
}

type Module struct {
	deps Deps
	repo *Repository
	svc  *Service
	ctrl *Controller
}

func NewModule(deps Deps) *Module {
	return &Module{deps: deps}
}

func (m *Module) Name() string { return "lms" }

func (m *Module) Setup() error {
	m.repo = NewRepository(m.deps.Mongo)
	var sheetsClient *storage.GoogleSheetsClient
	if m.deps.Cfg != nil && m.deps.Cfg.GDrive.CredentialJSON != "" {
		client, err := storage.NewGoogleSheetsClient(context.Background(), m.deps.Cfg.GDrive.CredentialJSON)
		if err != nil {
			if m.deps.Log != nil {
				m.deps.Log.Warn().Err(err).Msg("lms: google sheets client disabled")
			}
		} else {
			sheetsClient = client
		}
	}
	m.svc = NewService(m.repo, sheetsClient)
	m.ctrl = NewController(m.svc)
	if m.deps.Log != nil {
		m.deps.Log.Info().Msg("lms: module setup")
	}
	return nil
}

func (m *Module) RegisterRoutes(r *gin.Engine) {
	base := r.Group("/api/lms")
	if m.deps.JWTValidator != nil {
		base.Use(middleware.JWTMiddleware(m.deps.JWTValidator))
	}
	m.ctrl.RegisterRoutes(base)
}

func (m *Module) Stop(ctx context.Context) error {
	if m.deps.Log != nil {
		m.deps.Log.Info().Msg("lms: module stopped")
	}
	return nil
}
