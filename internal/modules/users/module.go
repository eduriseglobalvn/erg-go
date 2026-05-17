package users

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"

	usercontroller "erg.ninja/internal/modules/users/api/controller"
	userservice "erg.ninja/internal/modules/users/application/service"
	userrepo "erg.ninja/internal/modules/users/infrastructure/repository"
	"erg.ninja/pkg/auth"
	"erg.ninja/pkg/cache"
	"erg.ninja/pkg/config"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/storage"
	"erg.ninja/pkg/tenant"
)

// Module errors.
var (
	ErrForbidden = errors.New("users: forbidden")
)

// Deps holds all dependencies for the users module.
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
// Pattern: NewModule → Setup → RegisterRoutes → Stop.
type Module struct {
	deps Deps
	repo *userrepo.Repository
	svc  *userservice.Service
	ctrl *usercontroller.Controller
}

// NewModule creates a new users module instance.
func NewModule(deps Deps) *Module {
	return &Module{deps: deps}
}

// Name implements plugin.Module.
func (m *Module) Name() string { return "users" }

// Setup initialises the module.
func (m *Module) Setup() error {
	m.deps.Log.Info().Msg("users: module setup")
	m.repo = userrepo.NewRepository(m.deps.GORMClient)
	m.svc = userservice.New(m.repo, m.deps.R2, m.deps.Log, m.deps.Cfg)
	m.ctrl = usercontroller.New(m.svc, m.deps.JWTValidator, m.deps.Log)
	return nil
}

// RegisterRoutes mounts all /users routes.
func (m *Module) RegisterRoutes(r *gin.Engine) {
	if m.ctrl != nil {
		m.ctrl.RegisterRoutes(r)
	}
}

// Stop performs graceful shutdown.
func (m *Module) Stop(ctx context.Context) error {
	m.deps.Log.Info().Msg("users: module stopping")
	return nil
}
