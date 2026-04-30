package public_disclosure

import (
	"encoding/json"
	"fmt"

	"github.com/gin-gonic/gin"

	"erg.ninja/internal/dto/response"
	"erg.ninja/internal/middleware"
	docDto "erg.ninja/internal/modules/documents/dto"
	"erg.ninja/internal/modules/public_disclosure/entities"
	"erg.ninja/internal/modules/public_disclosure/service"
	"erg.ninja/pkg/auth"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/tenant"
)

type Controller struct {
	svc          *service.Service
	log          *logger.Logger
	jwtValidator *auth.JWTValidator
}

func NewController(svc *service.Service, log *logger.Logger, jwtValidator *auth.JWTValidator) *Controller {
	return &Controller{svc: svc, log: log, jwtValidator: jwtValidator}
}

func (c *Controller) RegisterRoutes(r *gin.Engine) {
	group := r.Group("/public-disclosure")
	group.GET("/", c.List)
	group.GET("/:id", c.GetByID)

	admin := group.Group("")
	admin.Use(middleware.JWTMiddleware(c.jwtValidator), middleware.RequireRoles("admin"))
	admin.POST("/", c.Create)
	admin.DELETE("/:id", c.Delete)
}

func (c *Controller) List(ctx *gin.Context) {
	tenantID := tenant.FromContext(ctx.Request.Context())
	section := ctx.Query("section")

	items, err := c.svc.List(ctx.Request.Context(), tenantID, section)
	if err != nil {
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, items)
}

func (c *Controller) Create(ctx *gin.Context) {
	tenantID := tenant.FromContext(ctx.Request.Context())

	// 1. Parse metadata from "data" field (JSON string)
	var doc entities.DisclosureDocument
	dataStr := ctx.Request.FormValue("data")
	if err := json.Unmarshal([]byte(dataStr), &doc); err != nil {
		response.BadRequestGin(ctx, fmt.Errorf("invalid metadata: %w", err))
		return
	}
	doc.TenantID = tenantID

	// 2. Parse watermark from "watermark" field
	var wmCfg docDto.WatermarkConfigDTO
	wmStr := ctx.Request.FormValue("watermark")
	if wmStr != "" {
		if err := json.Unmarshal([]byte(wmStr), &wmCfg); err != nil {
			response.BadRequestGin(ctx, fmt.Errorf("invalid watermark: %w", err))
			return
		}
	}

	// 3. Get file
	file, err := ctx.FormFile("file")
	if err != nil {
		response.BadRequestGin(ctx, fmt.Errorf("file is required: %w", err))
		return
	}

	res, err := c.svc.Create(ctx.Request.Context(), &doc, file, wmCfg)
	if err != nil {
		c.log.Error().Err(err).Msg("disclosure.ctrl.create")
		response.InternalErrorGin(ctx, err)
		return
	}

	response.CreatedGin(ctx, res)
}

func (c *Controller) GetByID(ctx *gin.Context) {
	tenantID := tenant.FromContext(ctx.Request.Context())
	id := ctx.Param("id")

	res, err := c.svc.GetByID(ctx.Request.Context(), tenantID, id)
	if err != nil {
		response.NotFoundGin(ctx, "disclosure not found")
		return
	}
	response.OKGin(ctx, res)
}

func (c *Controller) Delete(ctx *gin.Context) {
	tenantID := tenant.FromContext(ctx.Request.Context())
	id := ctx.Param("id")

	if err := c.svc.Delete(ctx.Request.Context(), tenantID, id); err != nil {
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, gin.H{"message": "deleted"})
}
