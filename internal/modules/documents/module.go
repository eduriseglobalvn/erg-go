package documents

import (
	"context"

	"github.com/gin-gonic/gin"

	documentcontroller "erg.ninja/internal/modules/documents/api/controller"
	documentservice "erg.ninja/internal/modules/documents/application/service"
	"erg.ninja/internal/modules/documents/infrastructure/repository"
	"erg.ninja/internal/modules/documents/infrastructure/watermark"
	"erg.ninja/pkg/auth"
	"erg.ninja/pkg/cache"
	"erg.ninja/pkg/config"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/storage"
	"erg.ninja/pkg/tenant"
)

type Service = documentservice.Service

type Deps struct {
	Mongo             *database.MongoClient
	Redis             *cache.RedisClient
	Log               *logger.Logger
	Cfg               *config.Config
	JWTValidator      *auth.JWTValidator
	TenantMongoClient *tenant.TenantMongoClient
	R2                *storage.R2Client
	GDrive            *storage.GDriveClient
}

type Module struct {
	deps     Deps
	repo     *repository.Repository
	svc      *documentservice.Service
	ctrl     *documentcontroller.Controller
	wmRender *watermark.Renderer
}

func NewModule(deps Deps) *Module {
	return &Module{deps: deps}
}

func NewController(svc *Service, log *logger.Logger, jwtValidator *auth.JWTValidator) *documentcontroller.Controller {
	return documentcontroller.NewController(svc, log, jwtValidator)
}

func ValidatePDFHeader(data []byte) bool {
	return documentservice.ValidatePDFHeader(data)
}

func sanitizeDocumentFilename(filename string) string {
	return documentservice.SanitizeDocumentFilename(filename)
}

func (m *Module) Name() string { return "documents" }

func (m *Module) Setup() error {
	if m.deps.Log != nil {
		m.deps.Log.Info().Msg("documents: module setup")
	}
	m.wmRender = watermark.NewRenderer(m.deps.Log)
	m.repo = repository.NewRepository(m.deps.Mongo, repository.WithDocumentsLogger(m.deps.Log))
	m.svc = documentservice.NewService(
		m.repo,
		m.deps.Redis,
		m.wmRender,
		m.deps.R2,
		m.deps.GDrive,
		documentservice.WithDocumentsLogger(m.deps.Log),
	)
	m.ctrl = documentcontroller.NewController(m.svc, m.deps.Log, m.deps.JWTValidator)
	return nil
}

func (m *Module) RegisterRoutes(r *gin.Engine) {
	if m.ctrl != nil {
		m.ctrl.RegisterRoutes(r)
	}
}

func (m *Module) Service() *documentservice.Service {
	return m.svc
}

func (m *Module) Stop(ctx context.Context) error {
	if m.deps.Log != nil {
		m.deps.Log.Info().Msg("documents: module stopped")
	}
	return nil
}
