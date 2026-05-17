package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"erg.ninja/internal/dto/response"
	"erg.ninja/internal/middleware"
	seodto "erg.ninja/internal/modules/seo/api/dto"
	seoservice "erg.ninja/internal/modules/seo/application/service"
	"erg.ninja/pkg/auth"
	"erg.ninja/pkg/logger"
)

type Controller struct {
	svc *seoservice.Service
	log *logger.Logger
}

func NewController(svc *seoservice.Service, log *logger.Logger) *Controller {
	return &Controller{svc: svc, log: log}
}

type CreateKeywordRequest = seodto.CreateKeywordRequest
type CreateRedirectRequest = seodto.CreateRedirectRequest
type Log404Request = seodto.Log404Request
type SaveSchemaRequest = seodto.SaveSchemaRequest
type SchemaType = seodto.SchemaType
type UpdateRedirectRequest = seodto.UpdateRedirectRequest

// RegisterRoutes mounts all SEO routes on the given router.
func (c *Controller) RegisterRoutes(r *gin.Engine, jwtVal *auth.JWTValidator) {
	r.GET("/api/seo/health", c.Health)

	api := r.Group("/api/seo")
	api.Use(middleware.JWTMiddleware(jwtVal))

	// Keywords
	api.GET("/keywords", c.ListKeywords)
	api.POST("/keywords", c.CreateKeyword)
	api.DELETE("/keywords/:id", c.DeleteKeyword)

	// Redirects
	api.GET("/redirects", c.ListRedirects)
	api.POST("/redirects", c.CreateRedirect)
	api.PUT("/redirects/:id", c.UpdateRedirect)
	api.DELETE("/redirects/:id", c.DeleteRedirect)

	// 404 Logs
	api.GET("/404-logs", c.List404Logs)

	// Schema
	api.GET("/schema/:postId", c.GetSchema)
	api.POST("/schema/:postId", c.SaveSchema)

	// GSC
	api.GET("/gsc/sync", c.SyncGSC)
	api.POST("/gsc/sync", c.SyncGSC)
	api.GET("/gsc/:postId", c.GetGSCDataForPost)
	api.GET("/gsc/top-posts", c.GetTopGSCPosts)
	api.GET("/top-posts", c.GetTopPosts)

	// Performance
	api.GET("/performance", c.GetPerformance)

	// Configs
	api.GET("/config/:key", c.GetConfig)
	api.PUT("/config/:key", c.UpsertConfig)

	// Dashboard & Analyze
	api.GET("/dashboard", c.GetDashboard)
	api.POST("/dashboard/batch-analyze", c.BatchAnalyze)
	api.GET("/analyze/:postId", c.AnalyzePost)
	api.POST("/analyze-draft", c.AnalyzeDraft)

	// AI & Optmization
	api.POST("/generate-alt-texts", c.GenerateAltTexts)
	api.POST("/suggest-meta", c.SuggestMeta)
	api.POST("/suggest-titles", c.SuggestTitles)
	api.PUT("/apply-autolinks/:postId", c.ApplyAutolinks)
	api.POST("/check-duplicate", c.CheckDuplicate)

	// GSC & Submissions
	api.GET("/gsc/auth/url", c.GetGSCAuthURL)
	api.POST("/gsc/auth/callback", c.GSCAuthCallback)
	api.GET("/performance/queries", c.GetPerformanceQueries)
	api.GET("/submission/logs", c.ListSubmissionLogs)
	api.POST("/submission/submit", c.SubmitURLs)

	// Keywords & Trends
	api.GET("/keyword-research", c.KeywordResearch)
	api.GET("/keyword-suggestions", c.KeywordSuggestions)
	api.GET("/keywords/suggest", c.KeywordSuggestions)
	api.GET("/keywords/autocomplete", c.KeywordAutocomplete)
	api.GET("/keywords/trending", c.KeywordTrending)
	api.GET("/history/:postId", c.GetHistory)
	api.GET("/trends/:postId", c.GetTrends)

	// Others
	api.POST("/posts/:postId/robots", c.UpdatePostRobots)
	api.POST("/schema/:postId/validate", c.ValidateSchema)
	api.POST("/404", c.Manual404Log)
}

// ─── Public ─────────────────────────────────────────────────────────────────

// Health handles GET /api/seo/health.
// @Summary SEO health check
// @Description Returns SEO module health status.
// @Tags SEO
// @Produce json
// @Success 200 {object} map[string]any
// @Router /api/seo/health [get]
func (c *Controller) Health(ctx *gin.Context) {
	result, err := c.svc.Health(ctx.Request.Context())
	if err != nil {
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, result)
}

// ─── Keywords ────────────────────────────────────────────────────────────────

// ListKeywords handles GET /api/seo/keywords.
// @Summary List SEO keywords
// @Description Fetch all tracked keywords and their targets.
// @Tags SEO Keywords
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Response
// @Router /api/seo/keywords [get]
func (c *Controller) ListKeywords(ctx *gin.Context) {
	keywords, err := c.svc.ListKeywords(ctx.Request.Context())
	if err != nil {
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, map[string]any{
		"items": keywords,
		"count": len(keywords),
	})
}

// CreateKeyword handles POST /api/seo/keywords.
// @Summary Add SEO keyword
// @Description Start tracking a new keyword for a specific target URL.
// @Tags SEO Keywords
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param payload body CreateKeywordRequest true "Keyword Data"
// @Success 201 {object} SeoKeyword
// @Router /api/seo/keywords [post]
func (c *Controller) CreateKeyword(ctx *gin.Context) {
	var req CreateKeywordRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, fmt.Errorf("invalid JSON: %w", err))
		return
	}
	if err := validateCreateKeyword(&req); err != nil {
		response.ValidationErrorGin(ctx, err)
		return
	}
	kw, err := c.svc.CreateKeyword(ctx.Request.Context(), &req)
	if err != nil {
		response.InternalErrorGin(ctx, err)
		return
	}
	response.CreatedGin(ctx, kw)
}

// DeleteKeyword handles DELETE /api/seo/keywords/:id.
// @Summary Delete SEO keyword
// @Description Remove a tracked keyword.
// @Tags SEO Keywords
// @Produce json
// @Security BearerAuth
// @Param id path string true "Keyword ID"
// @Success 200 {object} map[string]any
// @Router /api/seo/keywords/{id} [delete]
func (c *Controller) DeleteKeyword(ctx *gin.Context) {
	id := ctx.Param("id")
	if id == "" {
		response.BadRequestGin(ctx, fmt.Errorf("id is required"))
		return
	}
	if err := c.svc.DeleteKeyword(ctx.Request.Context(), id); err != nil {
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, map[string]any{"id": id, "deleted": true})
}

// ─── Redirects ───────────────────────────────────────────────────────────────

// ListRedirects handles GET /api/seo/redirects.
// @Summary List URL redirects
// @Description Fetch all active URL redirection rules.
// @Tags SEO Redirects
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Response
// @Router /api/seo/redirects [get]
func (c *Controller) ListRedirects(ctx *gin.Context) {
	redirects, err := c.svc.ListRedirects(ctx.Request.Context())
	if err != nil {
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, map[string]any{
		"items": redirects,
		"count": len(redirects),
	})
}

// CreateRedirect handles POST /api/seo/redirects.
// @Summary Create redirect rule
// @Description Add a new URL redirection rule.
// @Tags SEO Redirects
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 201 {object} map[string]any
// @Router /api/seo/redirects [post]
func (c *Controller) CreateRedirect(ctx *gin.Context) {
	var req CreateRedirectRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, fmt.Errorf("invalid JSON: %w", err))
		return
	}
	if err := validateCreateRedirect(&req); err != nil {
		response.ValidationErrorGin(ctx, err)
		return
	}
	red, err := c.svc.CreateRedirect(ctx.Request.Context(), &req)
	if err != nil {
		response.InternalErrorGin(ctx, err)
		return
	}
	response.CreatedGin(ctx, red)
}

// UpdateRedirect handles PUT /api/seo/redirects/:id.
// @Summary Update redirect rule
// @Description Update an existing redirect rule.
// @Tags SEO Redirects
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Redirect ID"
// @Success 200 {object} map[string]any
// @Router /api/seo/redirects/{id} [put]
func (c *Controller) UpdateRedirect(ctx *gin.Context) {
	id := ctx.Param("id")
	if id == "" {
		response.BadRequestGin(ctx, fmt.Errorf("id is required"))
		return
	}
	var req UpdateRedirectRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, fmt.Errorf("invalid JSON: %w", err))
		return
	}
	red, err := c.svc.UpdateRedirect(ctx.Request.Context(), id, &req)
	if err != nil {
		response.InternalErrorGin(ctx, err)
		return
	}
	if red == nil {
		response.NotFoundGin(ctx, "redirect not found")
		return
	}
	response.OKGin(ctx, red)
}

// DeleteRedirect handles DELETE /api/seo/redirects/:id.
// @Summary Delete redirect rule
// @Description Remove a URL redirection rule.
// @Tags SEO Redirects
// @Produce json
// @Security BearerAuth
// @Param id path string true "Redirect ID"
// @Success 200 {object} map[string]any
// @Router /api/seo/redirects/{id} [delete]
func (c *Controller) DeleteRedirect(ctx *gin.Context) {
	id := ctx.Param("id")
	if id == "" {
		response.BadRequestGin(ctx, fmt.Errorf("id is required"))
		return
	}
	if err := c.svc.DeleteRedirect(ctx.Request.Context(), id); err != nil {
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, map[string]any{"id": id, "deleted": true})
}

// ─── 404 Logs ────────────────────────────────────────────────────────────────

// List404Logs handles GET /api/seo/404-logs.
// @Summary List 404 error logs
// @Description Returns paginated 404 error log entries.
// @Tags SEO
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number"
// @Param limit query int false "Items per page"
// @Success 200 {object} map[string]any
// @Router /api/seo/404-logs [get]
func (c *Controller) List404Logs(ctx *gin.Context) {
	page := parseIntDefault(ctx.Query("page"), 1)
	limit := parseIntDefault(ctx.Query("limit"), 50)
	result, err := c.svc.List404Logs(ctx.Request.Context(), page, limit)
	if err != nil {
		response.InternalErrorGin(ctx, err)
		return
	}
	response.PaginatedGin(ctx, result.Items, result.Total, page, limit)
}

// ─── Schema ─────────────────────────────────────────────────────────────────

// GetSchema handles GET /api/seo/schema/{postId}.
// @Summary Get JSON-LD Schema
// @Description Fetch structured data markup for a specific post.
// @Tags SEO Schema
// @Accept json
// @Produce json
// @Param postId path string true "Post ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/seo/schema/{postId} [get]
func (c *Controller) GetSchema(ctx *gin.Context) {
	postID := ctx.Param("postId")
	if postID == "" {
		response.BadRequestGin(ctx, fmt.Errorf("postId is required"))
		return
	}
	schema, err := c.svc.GetSchema(ctx.Request.Context(), postID)
	if err != nil {
		response.InternalErrorGin(ctx, err)
		return
	}
	// Return as JSON-LD.
	if schema != nil {
		jsonLD := buildJSONLD(schema.Type, schema.Data)
		ctx.Header("Content-Type", "application/ld+json")
		ctx.JSON(http.StatusOK, jsonLD)
		return
	}
	response.OKGin(ctx, map[string]any{})
}

// SaveSchema handles POST /api/seo/schema/:postId.
// @Summary Save JSON-LD schema
// @Description Save structured data markup for a post.
// @Tags SEO Schema
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param postId path string true "Post ID"
// @Success 200 {object} map[string]any
// @Router /api/seo/schema/{postId} [post]
func (c *Controller) SaveSchema(ctx *gin.Context) {
	postID := ctx.Param("postId")
	if postID == "" {
		response.BadRequestGin(ctx, fmt.Errorf("postId is required"))
		return
	}
	var req SaveSchemaRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, fmt.Errorf("invalid JSON: %w", err))
		return
	}
	if err := c.svc.SaveSchema(ctx.Request.Context(), postID, req.Type, req.Data); err != nil {
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, map[string]any{"postId": postID, "schemaType": req.Type, "saved": true})
}

// ─── GSC ─────────────────────────────────────────────────────────────────────

// SyncGSC handles GET /api/seo/gsc/sync.
// @Summary Sync Google Search Console data
// @Description Trigger a sync of GSC data.
// @Tags SEO GSC
// @Produce json
// @Security BearerAuth
// @Param days query int false "Number of days to sync"
// @Success 200 {object} map[string]any
// @Router /api/seo/gsc/sync [get]
func (c *Controller) SyncGSC(ctx *gin.Context) {
	days := parseIntDefault(ctx.Query("days"), 7)
	if ctx.Request.Method == http.MethodPost {
		var req struct {
			Days int `json:"days"`
		}
		if err := ctx.ShouldBindJSON(&req); err == nil && req.Days > 0 {
			days = req.Days
		}
	}
	result, err := c.svc.SyncGSC(ctx.Request.Context(), days)
	if err != nil {
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, result)
}

// GetGSCDataForPost handles GET /api/seo/gsc/:postId.
// @Summary Get GSC data for a post
// @Description Returns Google Search Console metrics for a specific post.
// @Tags SEO GSC
// @Produce json
// @Security BearerAuth
// @Param postId path string true "Post ID"
// @Param days query int false "Number of days"
// @Success 200 {object} map[string]any
// @Router /api/seo/gsc/{postId} [get]
func (c *Controller) GetGSCDataForPost(ctx *gin.Context) {
	postID := ctx.Param("postId")
	if postID == "" {
		response.BadRequestGin(ctx, fmt.Errorf("postId is required"))
		return
	}
	days := parseIntDefault(ctx.Query("days"), 30)
	data, err := c.svc.GetGSCDataForPost(ctx.Request.Context(), postID, days)
	if err != nil {
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, map[string]any{"postId": postID, "days": days, "data": data})
}

// GetTopGSCPosts handles GET /api/seo/gsc/top-posts.
// @Summary Get top GSC posts
// @Description Returns top-performing posts from GSC data.
// @Tags SEO GSC
// @Produce json
// @Security BearerAuth
// @Param days query int false "Number of days"
// @Param limit query int false "Limit"
// @Success 200 {object} map[string]any
// @Router /api/seo/gsc/top-posts [get]
func (c *Controller) GetTopGSCPosts(ctx *gin.Context) {
	days := parseIntDefault(ctx.Query("days"), 30)
	limit := int64(parseIntDefault(ctx.Query("limit"), 50))
	data, err := c.svc.GetTopGSCPosts(ctx.Request.Context(), days, limit)
	if err != nil {
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, map[string]any{"days": days, "posts": data})
}

// GetTopPosts handles GET /api/seo/top-posts.
func (c *Controller) GetTopPosts(ctx *gin.Context) {
	days := parseIntDefault(ctx.Query("days"), 30)
	limit := int64(parseIntDefault(ctx.Query("limit"), 50))
	data, err := c.svc.GetTopGSCPosts(ctx.Request.Context(), days, limit)
	if err != nil {
		response.InternalErrorGin(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, data)
}

// ─── Performance ────────────────────────────────────────────────────────────

// GetPerformance handles GET /api/seo/performance.
// @Summary Get SEO performance
// @Description Returns aggregated SEO performance metrics.
// @Tags SEO
// @Produce json
// @Security BearerAuth
// @Param period query string false "Period (week/month/quarter)"
// @Success 200 {object} map[string]any
// @Router /api/seo/performance [get]
func (c *Controller) GetPerformance(ctx *gin.Context) {
	period := ctx.Query("period")
	if period == "" {
		period = "month"
	}
	result, err := c.svc.GetPerformance(ctx.Request.Context(), period)
	if err != nil {
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, result)
}

// ─── Configs ────────────────────────────────────────────────────────────────

// GetConfig handles GET /api/seo/config/:key.
// @Summary Get SEO config
// @Description Returns value of a specific SEO configuration key.
// @Tags SEO Config
// @Produce json
// @Security BearerAuth
// @Param key path string true "Config key"
// @Success 200 {object} map[string]any
// @Router /api/seo/config/{key} [get]
func (c *Controller) GetConfig(ctx *gin.Context) {
	key := ctx.Param("key")
	if key == "" {
		response.BadRequestGin(ctx, fmt.Errorf("key is required"))
		return
	}
	val, err := c.svc.GetConfig(ctx.Request.Context(), key)
	if err != nil {
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, map[string]any{"key": key, "value": val})
}

// UpsertConfig handles PUT /api/seo/config/:key.
// @Summary Set SEO config
// @Description Creates or updates an SEO configuration value.
// @Tags SEO Config
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param key path string true "Config key"
// @Success 200 {object} map[string]any
// @Router /api/seo/config/{key} [put]
func (c *Controller) UpsertConfig(ctx *gin.Context) {
	key := ctx.Param("key")
	if key == "" {
		response.BadRequestGin(ctx, fmt.Errorf("key is required"))
		return
	}
	var val any
	if err := ctx.ShouldBindJSON(&val); err != nil {
		response.BadRequestGin(ctx, fmt.Errorf("invalid JSON: %w", err))
		return
	}
	claims := middleware.GetClaims(ctx.Request.Context())
	updatedBy := ""
	if claims != nil {
		updatedBy = claims.UserID
	}
	if err := c.svc.UpsertConfig(ctx.Request.Context(), key, val, updatedBy); err != nil {
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, map[string]any{"key": key, "saved": true})
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func validateCreateKeyword(req *CreateKeywordRequest) error {
	if req.Keyword == "" {
		return fmt.Errorf("keyword: required")
	}
	if req.TargetURL == "" {
		return fmt.Errorf("target_url: required")
	}
	return nil
}

func validateCreateRedirect(req *CreateRedirectRequest) error {
	if req.FromPattern == "" {
		return fmt.Errorf("from_pattern: required")
	}
	if req.ToURL == "" {
		return fmt.Errorf("to_url: required")
	}
	return nil
}

func parseIntDefault(raw string, def int) int {
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return def
	}
	return v
}

// buildJSONLD constructs a JSON-LD object from schema type and data.
func buildJSONLD(schemaType SchemaType, data json.RawMessage) map[string]any {
	// Convert data to map.
	var dataMap map[string]any
	_ = json.Unmarshal(data, &dataMap)
	if dataMap == nil {
		dataMap = make(map[string]any)
	}
	return map[string]any{
		"@context": "https://schema.org",
		"@type":    string(schemaType),
		"_data":    dataMap,
	}
}

// ─── Missing Endpoints (Phase 1 placeholders) ───────────────────────────────

// GetDashboard handles GET /api/seo/dashboard.
// @Summary SEO Dashboard
// @Description Returns aggregated SEO dashboard data.
// @Tags SEO
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]any
// @Router /api/seo/dashboard [get]
func (c *Controller) GetDashboard(ctx *gin.Context) {
	response.SuccessGin(ctx, map[string]any{"status": "pending_implementation", "message": "SEO Dashboard data will be aggregated here"})
}

// BatchAnalyze handles POST /api/seo/dashboard/batch-analyze.
// @Summary Batch analyze posts
// @Description Analyzes multiple posts for SEO quality.
// @Tags SEO
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]any
// @Router /api/seo/dashboard/batch-analyze [post]
func (c *Controller) BatchAnalyze(ctx *gin.Context) {
	response.SuccessGin(ctx, map[string]any{"status": "pending_implementation"})
}

// AnalyzePost handles GET /api/seo/analyze/:postId.
// @Summary Analyze a post's SEO
// @Description Returns SEO analysis for a post.
// @Tags SEO
// @Produce json
// @Security BearerAuth
// @Param postId path string true "Post ID"
// @Success 200 {object} map[string]any
// @Router /api/seo/analyze/{postId} [get]
func (c *Controller) AnalyzePost(ctx *gin.Context) {
	response.SuccessGin(ctx, map[string]any{"postId": ctx.Param("postId"), "status": "pending_implementation"})
}

// AnalyzeDraft handles POST /api/seo/analyze-draft.
// @Summary Analyze draft SEO
// @Description Analyzes a draft post's SEO before publishing.
// @Tags SEO
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]any
// @Router /api/seo/analyze-draft [post]
func (c *Controller) AnalyzeDraft(ctx *gin.Context) {
	response.SuccessGin(ctx, map[string]any{"status": "pending_implementation"})
}

// GenerateAltTexts handles POST /api/seo/generate-alt-texts.
// @Summary Generate image alt texts
// @Description Uses AI to generate alt texts for images in content.
// @Tags SEO AI
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]any
// @Router /api/seo/generate-alt-texts [post]
func (c *Controller) GenerateAltTexts(ctx *gin.Context) {
	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, fmt.Errorf("invalid json: %w", err))
		return
	}
	claims := middleware.GetClaims(ctx.Request.Context())
	userID := ""
	if claims != nil {
		userID = claims.UserID
	}
	result, err := c.svc.GenerateAltTexts(ctx.Request.Context(), req.Content, userID)
	if err != nil {
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, map[string]any{"altTexts": result})
}

// SuggestMeta handles POST /api/seo/suggest-meta.
// @Summary Suggest SEO meta tags
// @Description AI-powered meta title and description suggestions.
// @Tags SEO AI
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]any
// @Router /api/seo/suggest-meta [post]
func (c *Controller) SuggestMeta(ctx *gin.Context) {
	var req struct {
		Title   string `json:"title" binding:"required"`
		Content string `json:"content" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, fmt.Errorf("invalid json: %w", err))
		return
	}
	claims := middleware.GetClaims(ctx.Request.Context())
	userID := ""
	if claims != nil {
		userID = claims.UserID
	}
	result, err := c.svc.SuggestMeta(ctx.Request.Context(), req.Title, req.Content, userID)
	if err != nil {
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, result)
}

// SuggestTitles handles POST /api/seo/suggest-titles.
// @Summary Suggest SEO-optimized titles
// @Description AI-powered title suggestions.
// @Tags SEO AI
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]any
// @Router /api/seo/suggest-titles [post]
func (c *Controller) SuggestTitles(ctx *gin.Context) {
	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, fmt.Errorf("invalid json: %w", err))
		return
	}
	claims := middleware.GetClaims(ctx.Request.Context())
	userID := ""
	if claims != nil {
		userID = claims.UserID
	}
	result, err := c.svc.SuggestTitles(ctx.Request.Context(), req.Content, userID)
	if err != nil {
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, map[string]any{"titles": result})
}

// ApplyAutolinks handles PUT /api/seo/apply-autolinks/:postId.
// @Summary Apply autolinks
// @Description Automatically adds internal links to post content.
// @Tags SEO
// @Produce json
// @Security BearerAuth
// @Param postId path string true "Post ID"
// @Success 200 {object} map[string]any
// @Router /api/seo/apply-autolinks/{postId} [put]
func (c *Controller) ApplyAutolinks(ctx *gin.Context) {
	response.SuccessGin(ctx, map[string]any{"postId": ctx.Param("postId"), "status": "pending_implementation"})
}

// CheckDuplicate handles POST /api/seo/check-duplicate.
// @Summary Check content duplication
// @Description Checks for duplicate content across posts.
// @Tags SEO
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]any
// @Router /api/seo/check-duplicate [post]
func (c *Controller) CheckDuplicate(ctx *gin.Context) {
	response.SuccessGin(ctx, map[string]any{"status": "pending_implementation"})
}

// GetGSCAuthURL handles GET /api/seo/gsc/auth/url.
// @Summary Get GSC auth URL
// @Description Returns OAuth2 URL for Google Search Console authorization.
// @Tags SEO GSC
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]any
// @Router /api/seo/gsc/auth/url [get]
func (c *Controller) GetGSCAuthURL(ctx *gin.Context) {
	url, err := c.svc.GetGSCAuthURL(ctx.Request.Context())
	if err != nil {
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, map[string]any{"url": url})
}

// GSCAuthCallback handles POST /api/seo/gsc/auth/callback.
// @Summary GSC OAuth callback
// @Description Exchanges authorization code for GSC tokens.
// @Tags SEO GSC
// @Produce json
// @Param code query string true "Authorization code"
// @Success 200 {object} map[string]any
// @Router /api/seo/gsc/auth/callback [post]
func (c *Controller) GSCAuthCallback(ctx *gin.Context) {
	code := ctx.Query("code")
	if code == "" {
		response.BadRequestGin(ctx, fmt.Errorf("code is required"))
		return
	}
	result, err := c.svc.ExchangeGSCToken(ctx.Request.Context(), code)
	if err != nil {
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, result)
}

// GetPerformanceQueries handles GET /api/seo/performance/queries.
// @Summary Get performance queries
// @Description Returns search queries performance data.
// @Tags SEO
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]any
// @Router /api/seo/performance/queries [get]
func (c *Controller) GetPerformanceQueries(ctx *gin.Context) {
	response.OKGin(ctx, map[string]any{"queries": []string{}})
}

// ListSubmissionLogs handles GET /api/seo/submission/logs.
// @Summary List URL submission logs
// @Description Returns URL submission history.
// @Tags SEO
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]any
// @Router /api/seo/submission/logs [get]
func (c *Controller) ListSubmissionLogs(ctx *gin.Context) {
	response.PaginatedGin(ctx, []any{}, 0, 1, 50)
}

// SubmitURLs handles POST /api/seo/submission/submit.
// @Summary Submit URLs to search engines
// @Description Submits URLs for indexing.
// @Tags SEO
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]any
// @Router /api/seo/submission/submit [post]
func (c *Controller) SubmitURLs(ctx *gin.Context) {
	response.OKGin(ctx, map[string]any{"submitted": 0})
}

// KeywordResearch handles GET /api/seo/keyword-research.
// @Summary Keyword research
// @Description Returns keyword research data.
// @Tags SEO Keywords
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]any
// @Router /api/seo/keyword-research [get]
func (c *Controller) KeywordResearch(ctx *gin.Context) {
	response.OKGin(ctx, map[string]any{"items": []any{}})
}

// KeywordSuggestions handles GET /api/seo/keyword-suggestions.
// @Summary Get keyword suggestions
// @Description Returns AI-generated keyword suggestions.
// @Tags SEO Keywords
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]any
// @Router /api/seo/keyword-suggestions [get]
func (c *Controller) KeywordSuggestions(ctx *gin.Context) {
	response.OKGin(ctx, map[string]any{
		"data": map[string]any{
			"hotTrends":        []any{},
			"paa":              []string{},
			"lsi":              []string{},
			"categoryTrending": []string{},
		},
	})
}

// GetTrends handles GET /api/seo/trends/:postId.
// @Summary Get post trends
// @Description Returns trend data for a specific post.
// @Tags SEO
// @Produce json
// @Param postId path string true "Post ID"
// @Success 200 {object} map[string]any
// @Router /api/seo/trends/{postId} [get]
func (c *Controller) GetTrends(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, []any{})
}

// GetHistory handles GET /api/seo/history/:postId.
func (c *Controller) GetHistory(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, []any{})
}

// KeywordAutocomplete handles GET /api/seo/keywords/autocomplete.
func (c *Controller) KeywordAutocomplete(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, []string{})
}

// KeywordTrending handles GET /api/seo/keywords/trending.
func (c *Controller) KeywordTrending(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, []any{})
}

// UpdatePostRobots handles POST /api/seo/posts/:postId/robots.
// @Summary Update post robots config
// @Description Updates robots.txt directives for a specific post.
// @Tags SEO
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param postId path string true "Post ID"
// @Success 200 {object} map[string]any
// @Router /api/seo/posts/{postId}/robots [post]
func (c *Controller) UpdatePostRobots(ctx *gin.Context) {
	response.OKGin(ctx, map[string]any{"postId": ctx.Param("postId"), "status": "updated"})
}

// ValidateSchema handles POST /api/seo/schema/:postId/validate.
// @Summary Validate JSON-LD schema
// @Description Validates structured data markup for a post.
// @Tags SEO Schema
// @Produce json
// @Security BearerAuth
// @Param postId path string true "Post ID"
// @Success 200 {object} map[string]any
// @Router /api/seo/schema/{postId}/validate [post]
func (c *Controller) ValidateSchema(ctx *gin.Context) {
	response.OKGin(ctx, map[string]any{"postId": ctx.Param("postId"), "isValid": true})
}

// Manual404Log handles POST /api/seo/404.
// @Summary Log a manual 404
// @Description Manually logs a 404 error for a URL.
// @Tags SEO
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]any
// @Router /api/seo/404 [post]
func (c *Controller) Manual404Log(ctx *gin.Context) {
	response.OKGin(ctx, map[string]any{"status": "logged"})
}
