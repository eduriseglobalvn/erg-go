package ai_content

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"

	aicontentcontroller "erg.ninja/internal/modules/ai_content/api/controller"
	aicontentdto "erg.ninja/internal/modules/ai_content/api/dto"
	aicontentservice "erg.ninja/internal/modules/ai_content/application/service"
	aicontentrepo "erg.ninja/internal/modules/ai_content/infrastructure/repository"
	"erg.ninja/pkg/ai"
	"erg.ninja/pkg/auth"
	"erg.ninja/pkg/cache"
	"erg.ninja/pkg/config"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/queue"
	"erg.ninja/pkg/security/secretbox"
	"erg.ninja/pkg/storage"
	"erg.ninja/pkg/tenant"
)

// Deps holds the ai_content module's dependencies.
type Deps struct {
	Mongo             *database.MongoClient
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
	repo *aicontentrepo.Repository
	q    *queue.AsynqClient
	svc  *aicontentservice.Service
	ctrl *aicontentcontroller.Controller
}

type RefineRequest = aicontentdto.RefineRequest
type Service = aicontentservice.Service

const TaskGeneratePost = aicontentservice.TaskGeneratePost

// NewModule creates a new ai_content module.
func NewModule(deps Deps) *Module { return &Module{deps: deps} }

// Name implements plugin.Module.
func (m *Module) Name() string { return "ai_content" }

// Service returns the underlying service instance.
func (m *Module) Service() *aicontentservice.Service { return m.svc }

// Setup initialises the module.
func (m *Module) Setup() error {
	m.deps.Log.Info().Msg("ai_content: module setup")

	qc, err := queue.NewAsynqClient(m.deps.Cfg.Queue, queue.WithAsynqClientLogger(m.deps.Log))
	if err != nil {
		return err
	}
	m.q = qc
	m.repo = aicontentrepo.NewRepository(m.deps.Mongo)

	aiClient, err := ai.NewClient(m.deps.Cfg.Ai, ai.WithGeminiLogger(m.deps.Log), ai.WithRedisCache(m.deps.Redis))
	if err != nil {
		return err
	}
	if aiClient.IsConfigured() {
		m.deps.Log.Info().Str("provider", aiClient.Provider()).Str("model", aiClient.Model()).Msg("ai_content: AI provider configured")
	} else {
		m.deps.Log.Warn().Str("provider", aiClient.Provider()).Str("model", aiClient.Model()).Msg("ai_content: AI provider has no API key")
	}
	imageClient := aicontentservice.NewHuggingFaceImageClient(m.deps.Cfg.Ai, m.deps.Log)
	if imageClient.IsConfigured() {
		m.deps.Log.Info().Str("model", imageClient.Model()).Msg("ai_content: Hugging Face image generation configured")
	} else {
		m.deps.Log.Warn().Str("model", imageClient.Model()).Msg("ai_content: Hugging Face image generation not configured")
	}
	var keyBox *secretbox.Box
	if m.deps.Cfg != nil && m.deps.Cfg.Ai.APIKeyEncryptionSecret != "" {
		keyBox, err = secretbox.New(m.deps.Cfg.Ai.APIKeyEncryptionSecret)
		if err != nil {
			return err
		}
		m.deps.Log.Info().Str("version", keyBox.Version()).Msg("ai_content: API key encryption configured")
	} else {
		m.deps.Log.Warn().Msg("ai_content: API key encryption secret not configured; database-managed AI keys are disabled")
	}
	m.svc = aicontentservice.NewService(m.repo, m.q, m.deps.Log, aiClient, m.deps.R2, imageClient, keyBox)
	if keyBox != nil {
		migrationCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		migrated, err := m.svc.MigratePlaintextKeys(migrationCtx)
		if err != nil {
			return err
		}
		if migrated > 0 {
			m.deps.Log.Warn().Int("count", migrated).Msg("ai_content: migrated plaintext API keys to encrypted storage")
		}
	}

	m.ctrl = aicontentcontroller.NewController(m.svc, m.deps.Log, m.deps.JWTValidator)
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
