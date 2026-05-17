package controller

import (
	"net/http"

	"erg.ninja/internal/persistence/postgrescore"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
	"github.com/gin-gonic/gin"
)

type Controller struct {
	pg  *database.GORMPostgresClient
	log *logger.Logger
}

func NewController(pg *database.GORMPostgresClient, log *logger.Logger) *Controller {
	return &Controller{
		pg:  pg,
		log: log,
	}
}

func (c *Controller) ListCenters(ctx *gin.Context) {
	if c.pg == nil {
		ctx.JSON(http.StatusServiceUnavailable, map[string]string{"error": "centers database is not configured"})
		return
	}
	var centers []postgrescore.Center
	if err := c.pg.DB().WithContext(ctx.Request.Context()).Order("type ASC, name ASC").Find(&centers).Error; err != nil {
		ctx.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to fetch centers"})
		return
	}

	ctx.JSON(http.StatusOK, map[string]any{
		"data": centers,
	})
}
