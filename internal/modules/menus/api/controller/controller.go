package controller

import (
	"github.com/gin-gonic/gin"

	"erg.ninja/internal/dto/response"
	menusservice "erg.ninja/internal/modules/menus/application/service"
	"erg.ninja/pkg/logger"
)

// Controller handles HTTP requests for the menus module.
type Controller struct {
	svc *menusservice.Service
	log *logger.Logger
}

// NewController creates a new menus controller.
func NewController(svc *menusservice.Service, log *logger.Logger) *Controller {
	return &Controller{svc: svc, log: log}
}

// RegisterRoutes mounts the menus REST API routes.
func (c *Controller) RegisterRoutes(r *gin.Engine) {
	menus := r.Group("/menus")
	menus.GET("/structure", c.GetStructure)
}

// GetStructure returns the navigation menu tree for a domain.
// @Summary Get menu structure
// @Description Returns the navigation menu tree for a domain.
// @Tags Menus
// @Produce json
// @Param domain query string false "Domain"
// @Success 200 {object} map[string]any
// @Router /menus/structure [get]
func (c *Controller) GetStructure(ctx *gin.Context) {
	domain := ctx.Query("domain")
	if domain == "" {
		domain = ctx.Request.Host
	}

	structure := c.svc.GetMenuStructure(domain)
	response.OKGin(ctx, structure)
}
