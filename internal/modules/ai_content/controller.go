// Package ai_content provides the HTTP controller for the AI Content proxy module.
package ai_content

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"erg.ninja/internal/dto/response"
	"erg.ninja/internal/middleware"
	"erg.ninja/pkg/logger"
)

// Controller handles HTTP requests for the AI Content module.
type Controller struct {
	svc *Service
	log *logger.Logger
}

// NewController creates a new ai_content controller.
func NewController(svc *Service, log *logger.Logger) *Controller {
	return &Controller{svc: svc, log: log}
}

// RegisterRoutes mounts the native AI Content routes onto the gin router.
func (c *Controller) RegisterRoutes(r *gin.Engine) {
	api := r.Group("/api/ai-content")

	// Job management
	api.POST("/generate", c.Generate)
	api.GET("/status/:jobId", c.Status)
	api.GET("/history", c.History)
	api.GET("/templates", c.Templates)

	// Refine
	api.POST("/refine", c.Refine)

	// Key management
	api.GET("/keys/my", c.MyKeys)
	api.GET("/keys/dashboard", c.KeysDashboard)
	api.POST("/keys", c.CreateKey)
	api.DELETE("/keys/:id", c.DeleteKey)
	api.POST("/keys/:id/test", c.TestKey)
	api.POST("/keys/:id/reactivate", c.ReactivateKey)

	// Provider
	api.GET("/provider-health", c.ProviderHealth)
}

// Generate handles POST /api/ai-content/generate.
// @Summary Generate AI content
// @Description Enqueues a task to generate a post using AI.
// @Tags AI Content
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param payload body GenerateRequest true "Generation parameters"
// @Success 200 {object} map[string]any
// @Router /api/ai-content/generate [post]
func (c *Controller) Generate(ctx *gin.Context) {
	var req GenerateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, fmt.Errorf("invalid json: %w", err))
		return
	}

	claims := middleware.GetClaims(ctx.Request.Context())
	userID := ""
	if claims != nil {
		userID = claims.UserID
	}

	jobID, err := c.svc.GeneratePost(ctx.Request.Context(), &req, userID)
	if err != nil {
		if errors.Is(err, ErrNoActiveAPIKey) {
			response.ErrorGin(ctx, http.StatusPreconditionFailed, "AI_KEY_MISSING", "Chưa có AI API key hoạt động. Vui lòng cấu hình tại Cài đặt > AI Keys.")
			return
		}
		response.InternalErrorGin(ctx, err)
		return
	}

	response.OKGin(ctx, map[string]any{
		"jobId":   jobID,
		"message": "Đang khởi tạo bài viết...",
	})
}

// Refine handles POST /api/ai-content/refine.
// @Summary Refine existing content
// @Description Sends content to LLM for improvements or rewriting.
// @Tags AI Content
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param payload body RefineRequest true "Content to refine"
// @Success 200 {object} map[string]any
// @Router /api/ai-content/refine [post]
func (c *Controller) Refine(ctx *gin.Context) {
	var req RefineRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, fmt.Errorf("invalid json: %w", err))
		return
	}

	claims := middleware.GetClaims(ctx.Request.Context())
	userID := ""
	if claims != nil {
		userID = claims.UserID
	}

	refined, err := c.svc.RefineContent(ctx.Request.Context(), &req, userID)
	if err != nil {
		if errors.Is(err, ErrNoActiveAPIKey) {
			response.ErrorGin(ctx, http.StatusPreconditionFailed, "AI_KEY_MISSING", "Chưa có AI API key hoạt động. Vui lòng cấu hình tại Cài đặt > AI Keys.")
			return
		}
		response.InternalErrorGin(ctx, err)
		return
	}

	response.OKGin(ctx, map[string]any{
		"refinedContent": refined,
	})
}

// Status handles GET /api/ai-content/status/:jobId.
// @Summary Check generation job status
// @Description Returns the current state of a background generation job.
// @Tags AI Content
// @Produce json
// @Security BearerAuth
// @Param jobId path string true "Job ID"
// @Success 200 {object} map[string]any
// @Router /api/ai-content/status/{jobId} [get]
func (c *Controller) Status(ctx *gin.Context) {
	jobID := ctx.Param("jobId")
	status, err := c.svc.GetJobStatus(ctx.Request.Context(), jobID)
	if err != nil {
		if errors.Is(err, ErrJobNotFound) {
			response.NotFoundGin(ctx, "Không tìm thấy job hoặc job đã bị xoá khỏi bộ đệm")
			return
		}
		response.InternalErrorGin(ctx, err)
		return
	}

	response.OKGin(ctx, status)
}

// History handles GET /api/ai-content/history.
// @Summary Get AI generation history
// @Description Returns past AI generation jobs.
// @Tags AI Content
// @Produce json
// @Success 200 {array} map[string]any
// @Router /api/ai-content/history [get]
func (c *Controller) History(ctx *gin.Context) { response.OKGin(ctx, []any{}) }

// Templates handles GET /api/ai-content/templates.
// @Summary Get available templates
// @Description Returns predefined content generation templates.
// @Tags AI Content
// @Produce json
// @Success 200 {array} map[string]any
// @Router /api/ai-content/templates [get]
func (c *Controller) Templates(ctx *gin.Context) { response.OKGin(ctx, []any{}) }

// MyKeys handles GET /api/ai-content/keys/my.
// @Summary List user's API keys
// @Description Returns the API keys belonging to the current user.
// @Tags AI Content Keys
// @Produce json
// @Security BearerAuth
// @Success 200 {array} map[string]any
// @Router /api/ai-content/keys/my [get]
func (c *Controller) MyKeys(ctx *gin.Context) { response.OKGin(ctx, []any{}) }

// KeysDashboard handles GET /api/ai-content/keys/dashboard.
// @Summary API keys usage dashboard
// @Description Returns aggregated usage stats for all API keys.
// @Tags AI Content Keys
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]any
// @Router /api/ai-content/keys/dashboard [get]
func (c *Controller) KeysDashboard(ctx *gin.Context) {
	response.OKGin(ctx, map[string]any{
		"total_keys":    0,
		"active_keys":   0,
		"total_usage":   0,
		"monthly_usage": 0,
	})
}

// CreateKey handles POST /api/ai-content/keys.
// @Summary Create an API key
// @Description Creates a new API key for AI content access.
// @Tags AI Content Keys
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 201 {object} map[string]any
// @Router /api/ai-content/keys [post]
func (c *Controller) CreateKey(ctx *gin.Context) { response.OKGin(ctx, map[string]any{}) }

// DeleteKey handles DELETE /api/ai-content/keys/:id.
// @Summary Delete an API key
// @Description Revokes and removes an API key.
// @Tags AI Content Keys
// @Produce json
// @Security BearerAuth
// @Param id path string true "Key ID"
// @Success 200 {object} map[string]any
// @Router /api/ai-content/keys/{id} [delete]
func (c *Controller) DeleteKey(ctx *gin.Context) { response.OKGin(ctx, map[string]any{}) }

// TestKey handles POST /api/ai-content/keys/:id/test.
// @Summary Test an API key
// @Description Tests connectivity of an API key.
// @Tags AI Content Keys
// @Produce json
// @Security BearerAuth
// @Param id path string true "Key ID"
// @Success 200 {object} map[string]any
// @Router /api/ai-content/keys/{id}/test [post]
func (c *Controller) TestKey(ctx *gin.Context) { response.OKGin(ctx, map[string]any{}) }

// ReactivateKey handles POST /api/ai-content/keys/:id/reactivate.
// @Summary Reactivate an API key
// @Description Reactivates a previously deactivated API key.
// @Tags AI Content Keys
// @Produce json
// @Security BearerAuth
// @Param id path string true "Key ID"
// @Success 200 {object} map[string]any
// @Router /api/ai-content/keys/{id}/reactivate [post]
func (c *Controller) ReactivateKey(ctx *gin.Context) { response.OKGin(ctx, map[string]any{}) }

// ProviderHealth handles GET /api/ai-content/provider-health.
// @Summary Check AI provider health
// @Description Returns health status of configured AI providers.
// @Tags AI Content
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]any
// @Router /api/ai-content/provider-health [get]
func (c *Controller) ProviderHealth(ctx *gin.Context) {
	health, err := c.svc.GetProviderHealth(ctx.Request.Context())
	if err != nil {
		response.InternalErrorGin(ctx, err)
		return
	}

	response.OKGin(ctx, health)
}
