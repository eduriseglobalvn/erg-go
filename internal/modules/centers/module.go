package centers

import (
	centerscontroller "erg.ninja/internal/modules/centers/api/controller"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
	"github.com/gin-gonic/gin"
)

type Module struct {
	controller *centerscontroller.Controller
}

type Deps struct {
	GORMClient *database.GORMPostgresClient
	Log        *logger.Logger
}

func NewModule(deps Deps) *Module {
	return &Module{
		controller: centerscontroller.NewController(deps.GORMClient, deps.Log),
	}
}

func (m *Module) Setup() error {
	return nil
}

func (m *Module) RegisterRoutes(r *gin.Engine) {
	api := r.Group("/api/v1/centers")
	{
		api.GET("", m.controller.ListCenters)
	}
}
