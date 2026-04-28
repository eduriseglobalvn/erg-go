package operations

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"erg.ninja/internal/dto/response"
	"erg.ninja/internal/middleware"
	"erg.ninja/pkg/auth"
	"erg.ninja/pkg/logger"
)

type Controller struct {
	svc *Service
	log *logger.Logger
}

func NewController(svc *Service, log *logger.Logger) *Controller {
	return &Controller{svc: svc, log: log}
}

func (c *Controller) RegisterRoutes(r *gin.Engine, jwtVal *auth.JWTValidator) {
	// r.GET("/api/health", c.GetSystemStatus) // Redundant, registered in routes.go

	api := r.Group("/api/operations")
	if jwtVal != nil {
		api.Use(middleware.JWTMiddleware(jwtVal))
	}

	api.GET("/system-status", c.GetSystemStatus)
	api.GET("/logs", c.ListLogs)

	// Firewall
	api.GET("/firewall/list", c.ListBlockedIPs)
	api.POST("/firewall/block", c.BlockIP)
	api.POST("/firewall/unblock", c.UnblockIP)
	api.GET("/firewall/check/:ip", c.CheckIP)

	// Configs
	api.GET("/configs", c.ListConfigs)
	api.GET("/configs/:key", c.GetConfig)
	api.PUT("/configs/:key", c.SetConfig)
	api.DELETE("/configs/:key", c.DeleteConfig)

	configCompat := r.Group("/api/operations/config")
	if jwtVal != nil {
		configCompat.Use(middleware.JWTMiddleware(jwtVal))
	}
	configCompat.GET("", c.ListConfigs)
	configCompat.GET("/:key", c.GetConfig)
	configCompat.PUT("/:key", c.SetConfig)
	configCompat.DELETE("/:key", c.DeleteConfig)

	ipCompat := r.Group("/api/admin/ip")
	if jwtVal != nil {
		ipCompat.Use(middleware.JWTMiddleware(jwtVal))
	}
	ipCompat.POST("/block", c.BlockIP)
	ipCompat.DELETE("/unblock/:ip", c.UnblockIPByParam)
	ipCompat.GET("/check/:ip", c.CheckIP)
}

// GetSystemStatus handles GET /api/operations/system-status.
// @Summary Get system health status
// @Description Returns CPU, RAM, and Database connectivity status.
// @Tags Operations
// @Produce json
// @Security BearerAuth
// @Success 200 {object} SystemStatus
// @Router /api/operations/system-status [get]
func (c *Controller) GetSystemStatus(ctx *gin.Context) {
	status, err := c.svc.GetSystemStatus(ctx.Request.Context())
	if err != nil {
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, status)
}

// ListLogs handles GET /api/operations/logs.
// @Summary List audit logs
// @Description Retrieves paginated audit logs from MongoDB.
// @Tags Operations
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number"
// @Param limit query int false "Items per page"
// @Success 200 {object} response.Response{data=[]map[string]any}
// @Router /api/operations/logs [get]
func (c *Controller) ListLogs(ctx *gin.Context) {
	page, _ := ctx.GetQuery("page")
	limit, _ := ctx.GetQuery("limit")

	p := 1
	l := 20

	// Simple parsing
	if page != "" {
		fmt.Sscanf(page, "%d", &p)
	}
	if limit != "" {
		fmt.Sscanf(limit, "%d", &l)
	}

	logs, total, err := c.svc.ListLogs(ctx.Request.Context(), p, l)
	if err != nil {
		response.InternalErrorGin(ctx, err)
		return
	}
	response.PaginatedGin(ctx, logs, total, p, l)
}

// BlockIP handles POST /api/operations/firewall/block.
// @Summary Block an IP address
// @Description Adds an IP to the Redis blacklist with optional duration.
// @Tags Operations Firewall
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param payload body map[string]any true "IP and Duration"
// @Success 200 {object} map[string]any
// @Router /api/operations/firewall/block [post]
func (c *Controller) BlockIP(ctx *gin.Context) {
	var req struct {
		IP       string `json:"ip" binding:"required"`
		Duration int    `json:"duration_seconds"` // optional
		TTLMS    int    `json:"ttlMs"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}

	duration := time.Duration(req.Duration) * time.Second
	if req.TTLMS > 0 {
		duration = time.Duration(req.TTLMS) * time.Millisecond
	}
	if err := c.svc.BlockIP(ctx.Request.Context(), req.IP, duration); err != nil {
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, map[string]any{"ip": req.IP, "status": "blocked"})
}

// UnblockIP handles POST /api/operations/firewall/unblock.
// @Summary Unblock an IP address
// @Description Removes an IP from the Redis blacklist.
// @Tags Operations Firewall
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param payload body map[string]any true "IP Address"
// @Success 200 {object} map[string]any
// @Router /api/operations/firewall/unblock [post]
func (c *Controller) UnblockIP(ctx *gin.Context) {
	var req struct {
		IP string `json:"ip" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}

	if err := c.svc.UnblockIP(ctx.Request.Context(), req.IP); err != nil {
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, map[string]any{"ip": req.IP, "status": "unblocked"})
}

// UnblockIPByParam handles DELETE /api/admin/ip/unblock/:ip.
func (c *Controller) UnblockIPByParam(ctx *gin.Context) {
	ip := ctx.Param("ip")
	if ip == "" {
		response.BadRequestGin(ctx, fmt.Errorf("ip is required"))
		return
	}
	if err := c.svc.UnblockIP(ctx.Request.Context(), ip); err != nil {
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, map[string]any{"ip": ip, "status": "unblocked"})
}

// ListBlockedIPs handles GET /api/operations/firewall/list.
// @Summary List all blocked IPs
// @Description Returns a list of currently blocked IPs and their TTL.
// @Tags Operations Firewall
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Response{data=[]map[string]any}
// @Router /api/operations/firewall/list [get]
func (c *Controller) ListBlockedIPs(ctx *gin.Context) {
	list, err := c.svc.ListBlockedIPs(ctx.Request.Context())
	if err != nil {
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, list)
}

// ListConfigs handles GET /api/operations/configs.
// @Summary List system configurations
// @Description Returns all global system configuration keys and values.
// @Tags Operations Config
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]any
// @Router /api/operations/configs [get]
func (c *Controller) ListConfigs(ctx *gin.Context) {
	list, err := c.svc.ListConfigs(ctx.Request.Context())
	if err != nil {
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, list)
}

// GetConfig handles GET /api/operations/configs/:key.
// @Summary Get a system configuration
// @Description Returns a specific configuration value by key.
// @Tags Operations Config
// @Produce json
// @Security BearerAuth
// @Param key path string true "Config Key"
// @Success 200 {object} map[string]any
// @Router /api/operations/configs/{key} [get]
func (c *Controller) GetConfig(ctx *gin.Context) {
	key := ctx.Param("key")
	cfg, err := c.svc.GetConfig(ctx.Request.Context(), key)
	if err != nil {
		response.InternalErrorGin(ctx, err)
		return
	}
	if cfg == nil {
		response.NotFoundGin(ctx, "Config not found")
		return
	}
	response.OKGin(ctx, cfg)
}

// SetConfig handles PUT /api/operations/configs/:key.
// @Summary Update a system configuration
// @Description Sets or updates a configuration value.
// @Tags Operations Config
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param key path string true "Config Key"
// @Param payload body map[string]any true "Config Data"
// @Success 200 {object} map[string]any
// @Router /api/operations/configs/{key} [put]
func (c *Controller) SetConfig(ctx *gin.Context) {
	key := ctx.Param("key")
	var req struct {
		Value       any    `json:"value" binding:"required"`
		Description string `json:"description"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}

	claims := middleware.GetClaims(ctx.Request.Context())
	userID := ""
	if claims != nil {
		userID = claims.UserID
	}

	if err := c.svc.SetConfig(ctx.Request.Context(), key, req.Value, req.Description, userID); err != nil {
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, map[string]any{"key": key, "status": "updated"})
}

// CheckIP handles GET /api/operations/firewall/check/:ip.
// @Summary Check if an IP is blocked
// @Description Returns the blocked status and TTL for a specific IP.
// @Tags Operations Firewall
// @Produce json
// @Security BearerAuth
// @Param ip path string true "IP Address"
// @Success 200 {object} map[string]any
// @Router /api/operations/firewall/check/{ip} [get]
func (c *Controller) CheckIP(ctx *gin.Context) {
	ip := ctx.Param("ip")
	blocked, err := c.svc.IsIPBlocked(ctx.Request.Context(), ip)
	if err != nil {
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, map[string]any{"ip": ip, "blocked": blocked, "isBlocked": blocked})
}

// DeleteConfig handles DELETE /api/operations/configs/:key.
// @Summary Delete a system configuration
// @Description Removes a configuration key from the system.
// @Tags Operations Config
// @Produce json
// @Security BearerAuth
// @Param key path string true "Config Key"
// @Success 200 {object} map[string]any
// @Router /api/operations/configs/{key} [delete]
func (c *Controller) DeleteConfig(ctx *gin.Context) {
	key := ctx.Param("key")
	if err := c.svc.DeleteConfig(ctx.Request.Context(), key); err != nil {
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, map[string]any{"key": key, "status": "deleted"})
}
