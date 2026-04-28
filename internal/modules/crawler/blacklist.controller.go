// Package crawler provides blacklist CRUD controllers.
package crawler

import (
	"erg.ninja/internal/dto/response"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"

	"erg.ninja/internal/modules/crawler/dto"
	"erg.ninja/internal/modules/crawler/entities"
	"erg.ninja/internal/modules/crawler/repository"
	"erg.ninja/pkg/logger"
)

// BlacklistController handles HTTP requests for content blacklist management.
type BlacklistController struct {
	repo *repository.Repository
	log  *logger.Logger
}

// NewBlacklistController creates a new blacklist controller.
func NewBlacklistController(repo *repository.Repository, log *logger.Logger) *BlacklistController {
	return &BlacklistController{repo: repo, log: log}
}

// RegisterRoutes mounts the blacklist REST API routes.
func (c *BlacklistController) RegisterRoutes(r *gin.Engine) {
	api := r.Group("/api/blacklist")
	api.POST("/", c.CreateEntry)
	api.GET("/", c.ListEntries)
	api.DELETE("/:id", c.DeleteEntry)

	legacy := r.Group("/api/crawler/blacklist")
	legacy.GET("", c.ListEntriesCompat)
	legacy.POST("", c.CreateEntryCompat)
	legacy.PATCH("/:id", c.UpdateEntryCompat)
	legacy.DELETE("/:id", c.DeleteEntryCompat)
}

// CreateEntry handles POST /api/blacklist.
// @Summary Create blacklist entry
// @Description Adds a new blacklist pattern.
// @Tags Crawler Blacklist
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 201 {object} map[string]any
// @Router /api/blacklist [post]
func (c *BlacklistController) CreateEntry(ctx *gin.Context) {
	var req struct {
		dto.CreateBlacklistRequest
		Value string `json:"value"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		c.writeError(ctx, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}

	pattern := req.Pattern
	if pattern == "" {
		pattern = req.Value
	}
	if pattern == "" {
		c.writeError(ctx, http.StatusBadRequest, "MISSING_PATTERN", "pattern is required")
		return
	}

	blType := entities.BlacklistType(req.Type)
	if blType == "" {
		blType = entities.BlacklistURL
	}

	entry := &entities.ContentBlacklist{
		Type:    blType,
		Pattern: pattern,
		Reason:  req.Reason,
		Enabled: true,
	}

	if err := c.repo.CreateBlacklistEntry(ctx.Request.Context(), entry); err != nil {
		c.writeError(ctx, http.StatusInternalServerError, "CREATE_FAILED", err.Error())
		return
	}

	c.writeJSON(ctx, http.StatusCreated, dto.BlacklistToResponse(entry))
}

// ListEntries handles GET /api/blacklist.
// @Summary List blacklist entries
// @Description Returns all blacklist entries.
// @Tags Crawler Blacklist
// @Produce json
// @Param type query string false "Type filter"
// @Param enabled query string false "Enabled filter"
// @Success 200 {object} map[string]any
// @Router /api/blacklist [get]
func (c *BlacklistController) ListEntries(ctx *gin.Context) {
	var blType *entities.BlacklistType
	if t := ctx.Query("type"); t != "" {
		bt := entities.BlacklistType(t)
		blType = &bt
	}

	var enabled *bool
	if s := ctx.Query("enabled"); s != "" {
		b := s == "true"
		enabled = &b
	}

	entries, err := c.repo.ListBlacklist(ctx.Request.Context(), blType, enabled)
	if err != nil {
		c.writeError(ctx, http.StatusInternalServerError, "LIST_FAILED", err.Error())
		return
	}

	c.writeJSON(ctx, http.StatusOK, map[string]any{
		"items": dto.BlacklistsToResponses(entries),
		"total": len(entries),
	})
}

// DeleteEntry handles DELETE /api/blacklist/:id.
// @Summary Delete blacklist entry
// @Description Removes a blacklist entry.
// @Tags Crawler Blacklist
// @Produce json
// @Security BearerAuth
// @Param id path string true "Entry ID"
// @Success 200 {object} map[string]any
// @Router /api/blacklist/{id} [delete]
func (c *BlacklistController) DeleteEntry(ctx *gin.Context) {
	id := ctx.Param("id")

	if err := c.repo.DeleteBlacklistEntry(ctx.Request.Context(), id); err != nil {
		c.writeError(ctx, http.StatusInternalServerError, "DELETE_FAILED", err.Error())
		return
	}

	c.writeJSON(ctx, http.StatusOK, map[string]string{
		"status": "deleted",
		"id":     id,
	})
}

func (c *BlacklistController) ListEntriesCompat(ctx *gin.Context) {
	var blType *entities.BlacklistType
	if t := ctx.Query("type"); t != "" {
		bt := entities.BlacklistType(t)
		blType = &bt
	}

	entries, err := c.repo.ListBlacklist(ctx.Request.Context(), blType, nil)
	if err != nil {
		c.writeError(ctx, http.StatusInternalServerError, "LIST_FAILED", err.Error())
		return
	}

	items := make([]blacklistCompatResponse, 0, len(entries))
	for _, entry := range entries {
		items = append(items, newBlacklistCompatResponse(entry))
	}
	c.writeJSON(ctx, http.StatusOK, items)
}

func (c *BlacklistController) CreateEntryCompat(ctx *gin.Context) {
	var req struct {
		Type      string `json:"type"`
		Value     string `json:"value"`
		Reason    string `json:"reason"`
		CreatedBy string `json:"createdBy"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		c.writeError(ctx, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}

	if req.Value == "" {
		c.writeError(ctx, http.StatusBadRequest, "MISSING_VALUE", "value is required")
		return
	}

	blType := entities.BlacklistType(req.Type)
	if blType == "" {
		blType = entities.BlacklistDomain
	}

	entry := &entities.ContentBlacklist{
		Type:      blType,
		Pattern:   req.Value,
		Reason:    req.Reason,
		CreatedBy: req.CreatedBy,
		Enabled:   true,
	}
	if err := c.repo.CreateBlacklistEntry(ctx.Request.Context(), entry); err != nil {
		c.writeError(ctx, http.StatusInternalServerError, "CREATE_FAILED", err.Error())
		return
	}

	c.writeJSON(ctx, http.StatusCreated, newBlacklistCompatResponse(entry))
}

func (c *BlacklistController) UpdateEntryCompat(ctx *gin.Context) {
	id := ctx.Param("id")
	var req struct {
		Reason   string `json:"reason"`
		IsActive *bool  `json:"isActive"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		c.writeError(ctx, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}

	update := bson.M{}
	if req.Reason != "" {
		update["reason"] = req.Reason
	}
	if req.IsActive != nil {
		update["enabled"] = *req.IsActive
	}
	if err := c.repo.UpdateBlacklistEntry(ctx.Request.Context(), id, update); err != nil {
		c.writeError(ctx, http.StatusInternalServerError, "UPDATE_FAILED", err.Error())
		return
	}

	entry, err := c.repo.GetBlacklistEntry(ctx.Request.Context(), id)
	if err != nil {
		c.writeError(ctx, http.StatusInternalServerError, "GET_FAILED", err.Error())
		return
	}
	if entry == nil {
		c.writeError(ctx, http.StatusNotFound, "NOT_FOUND", "blacklist entry not found")
		return
	}

	c.writeJSON(ctx, http.StatusOK, newBlacklistCompatResponse(entry))
}

func (c *BlacklistController) DeleteEntryCompat(ctx *gin.Context) {
	id := ctx.Param("id")
	if err := c.repo.DeleteBlacklistEntry(ctx.Request.Context(), id); err != nil {
		c.writeError(ctx, http.StatusInternalServerError, "DELETE_FAILED", err.Error())
		return
	}

	c.writeJSON(ctx, http.StatusOK, map[string]any{"success": true})
}

// ─── Helpers ─────────────────────────────────────────────────────────────

func (c *BlacklistController) writeJSON(ctx *gin.Context, status int, v any) {
	response.WriteGin(ctx, status, v, nil, nil)
}

func (c *BlacklistController) writeError(ctx *gin.Context, status int, code, message string) {
	response.ErrorGin(ctx, status, code, message)
}
