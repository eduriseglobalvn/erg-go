// Package controller handles HTTP requests for the analytics module.
package controller

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"erg.ninja/internal/dto/response"
	"erg.ninja/internal/middleware"
	"erg.ninja/internal/modules/analytics/api/dto"
	"erg.ninja/internal/modules/analytics/application/service"
	"erg.ninja/pkg/auth"
	"erg.ninja/pkg/config"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/tenant"
)

// Controller handles HTTP requests for analytics.
type Controller struct {
	svc          *service.Service
	log          *logger.Logger
	cfg          *config.Config
	jwtValidator *auth.JWTValidator
}

// NewController creates a new analytics controller.
func NewController(svc *service.Service, log *logger.Logger, cfg *config.Config, jwtValidator *auth.JWTValidator) *Controller {
	return &Controller{svc: svc, log: log, cfg: cfg, jwtValidator: jwtValidator}
}

// RegisterRoutes mounts the analytics REST API routes onto the router.
func (c *Controller) RegisterRoutes(r *gin.Engine) {
	api := r.Group("/api/insight")
	{
		// ── Public tracking endpoints (no auth required) ──────────────────
		api.POST("/session/begin", c.TrackVisit)
		api.POST("/behavior", c.TrackEvent)
		api.PUT("/session/:id/finish", c.FinishSession)
		api.POST("/identify", c.Identify)

		// ── Firebase sync (internal, API key protected) ─────────────────
		fb := api.Group("/firebase")
		fb.Use(c.firebaseAuthMiddleware())
		{
			fb.POST("/sync", c.SyncFirebaseEvents)
		}

		// ── Protected dashboard endpoints (auth required) ─────────────────
		protected := api.Group("/")
		protected.Use(middleware.JWTMiddleware(c.jwtValidator))
		{
			read := protected.Group("/")
			read.Use(middleware.RequirePermission("analytics.read"))
			{
				read.GET("/stats", c.GetStats)
				read.GET("/overview", c.GetOverview)
				read.GET("/posts/summary", c.GetPostSummary)
				read.GET("/top-content", c.GetTopContent)
				read.GET("/traffic-sources", c.GetTrafficSources)
				read.GET("/insights", c.GetInsights)
				read.GET("/user-journey/:sessionId", c.GetUserJourney)
				read.GET("/sessions/:sessionId", c.GetSession)
				read.GET("/firebase/stats", c.GetFirebaseSyncStats)
			}
			protected.GET("/export", middleware.RequirePermission("analytics.export"), c.Export)
		}
	}
}

// ─── Public Tracking Handlers ─────────────────────────────────────────────────

// TrackVisit handles POST /api/insight/session/begin — no auth required.
// @Summary Begin session tracking
// @Description Register a new user session visit.
// @Tags Insight
// @Accept json
// @Produce json
// @Param payload body dto.TrackVisitRequest true "Visit Data"
// @Success 200 {object} dto.TrackVisitResponse
// @Failure 400 {object} response.Response
// @Router /api/insight/session/begin [post]
func (c *Controller) TrackVisit(ctx *gin.Context) {
	tenantID := tenant.FromContext(ctx.Request.Context())

	var req dto.TrackVisitRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	if req.URL == "" {
		response.BadRequestGin(ctx, fmt.Errorf("url is required"))
		return
	}

	ip := extractRealIP(ctx.Request)
	userAgent := ctx.GetHeader("User-Agent")
	var userID *int64

	resp, err := c.svc.TrackVisit(ctx.Request.Context(), tenantID, req, ip, userAgent, userID)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("analytics: TrackVisit failed")
		response.InternalErrorGin(ctx, err)
		return
	}

	response.SuccessGin(ctx, resp)
}

// TrackEvent handles POST /api/insight/behavior — no auth required.
// @Summary Track user behavior
// @Description Track an event linked to a session.
// @Tags Insight
// @Accept json
// @Produce json
// @Param payload body dto.TrackEventRequest true "Event Data"
// @Success 200 {object} map[string]string
// @Router /api/insight/behavior [post]
func (c *Controller) TrackEvent(ctx *gin.Context) {
	tenantID := tenant.FromContext(ctx.Request.Context())

	var req dto.TrackEventRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	if req.SessionInternalID == "" || req.EventType == "" {
		response.BadRequestGin(ctx, fmt.Errorf("session_id and event_type are required"))
		return
	}

	var userID *int64
	if err := c.svc.TrackEvent(ctx.Request.Context(), tenantID, req, userID); err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("analytics: TrackEvent failed")
		response.InternalErrorGin(ctx, err)
		return
	}

	response.SuccessGin(ctx, dto.TrackEventResponse{
		Success:   true,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// FinishSession handles PUT /api/insight/session/:id/finish — no auth required.
// @Summary Finish session
// @Description Marks a session as finished with duration.
// @Tags Insight
// @Accept json
// @Produce json
// @Param id path string true "Session ID"
// @Success 200 {object} map[string]any
// @Router /api/insight/session/{id}/finish [put]
func (c *Controller) FinishSession(ctx *gin.Context) {
	id := ctx.Param("id")
	if id == "" {
		response.BadRequestGin(ctx, fmt.Errorf("id is required"))
		return
	}

	var req dto.FinishSessionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}

	if err := c.svc.FinishSession(ctx.Request.Context(), id, req.Duration); err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("analytics: FinishSession failed")
		response.InternalErrorGin(ctx, err)
		return
	}

	response.SuccessGin(ctx, map[string]any{"success": true})
}

// Identify handles POST /api/insight/identify — no auth required (called after login).
// @Summary Identify user
// @Description Links a session to a user ID after login.
// @Tags Insight
// @Accept json
// @Produce json
// @Success 200 {object} map[string]any
// @Router /api/insight/identify [post]
func (c *Controller) Identify(ctx *gin.Context) {
	tenantID := tenant.FromContext(ctx.Request.Context())

	var req dto.IdentifyRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	if req.SessionID == "" || req.UserID == 0 {
		response.BadRequestGin(ctx, fmt.Errorf("session_id and user_id are required"))
		return
	}

	if err := c.svc.Identify(ctx.Request.Context(), tenantID, req.SessionID, req.UserID); err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("analytics: Identify failed")
		response.InternalErrorGin(ctx, err)
		return
	}

	response.SuccessGin(ctx, map[string]any{"success": true})
}

// ─── Protected Dashboard Handlers ────────────────────────────────────────────

// GetStats handles GET /api/insight/stats — requires JWT auth + system.logs permission.
// @Summary General statistics
// @Description Get overall analytics statistics.
// @Tags Insight
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {array} dto.VisitorStat
// @Failure 401 {object} response.Response
// @Router /api/insight/stats [get]
func (c *Controller) GetStats(ctx *gin.Context) {
	params := dto.VisitorStatsParams{
		Range: ctx.Query("range"),
		From:  ctx.Query("from"),
		To:    ctx.Query("to"),
	}
	if params.Range == "" {
		params.Range = "7d"
	}
	validRanges := map[string]bool{"7d": true, "30d": true, "90d": true}
	if params.From == "" && !validRanges[params.Range] {
		response.BadRequestGin(ctx, fmt.Errorf("invalid range: %s", params.Range))
		return
	}

	stats, err := c.svc.GetVisitorStats(ctx.Request.Context(), params)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("analytics: GetStats failed")
		response.InternalErrorGin(ctx, err)
		return
	}

	response.SuccessGin(ctx, stats)
}

// GetOverview handles GET /api/insight/overview — requires JWT auth + system.logs.
// @Summary Dashboard overview
// @Description Get comprehensive analytics overview for the dashboard.
// @Tags Insight
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param from query string false "Start date (ISO)"
// @Param to query string false "End date (ISO)"
// @Success 200 {object} dto.DashboardOverviewResponse
// @Router /api/insight/overview [get]
func (c *Controller) GetOverview(ctx *gin.Context) {
	params := dto.OverviewParams{
		From: ctx.Query("from"),
		To:   ctx.Query("to"),
	}

	overview, err := c.svc.GetOverview(ctx.Request.Context(), params)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("analytics: GetOverview failed")
		response.InternalErrorGin(ctx, err)
		return
	}

	response.SuccessGin(ctx, overview)
}

// GetPostSummary handles GET /api/insight/posts/summary.
// @Summary Post analytics summary
// @Description Returns post-level analytics summary.
// @Tags Insight
// @Produce json
// @Security BearerAuth
// @Param range query string false "Range (7d/30d/90d)"
// @Success 200 {object} map[string]any
// @Router /api/insight/posts/summary [get]
func (c *Controller) GetPostSummary(ctx *gin.Context) {
	rangeStr := ctx.Query("range")
	if rangeStr == "" {
		rangeStr = "90d"
	}

	summary, err := c.svc.GetPostSummary(ctx.Request.Context(), rangeStr)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("analytics: GetPostSummary failed")
		response.InternalErrorGin(ctx, err)
		return
	}

	response.SuccessGin(ctx, summary)
}

// GetTopContent handles GET /api/insight/top-content.
// @Summary Top content
// @Description Returns top pages, posts, and courses.
// @Tags Insight
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]any
// @Router /api/insight/top-content [get]
func (c *Controller) GetTopContent(ctx *gin.Context) {
	// Top content is embedded in the overview response.
	// This endpoint mirrors the NestJS endpoint for completeness.
	params := dto.OverviewParams{
		From: ctx.Query("from"),
		To:   ctx.Query("to"),
	}
	overview, err := c.svc.GetOverview(ctx.Request.Context(), params)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("analytics: GetTopContent failed")
		response.InternalErrorGin(ctx, err)
		return
	}

	response.SuccessGin(ctx, map[string]any{
		"topPages":   overview.Content.TopPages,
		"topCourses": overview.Content.TopCourses,
		"topPosts":   overview.Content.TopPosts,
	})
}

// GetTrafficSources handles GET /api/insight/traffic-sources.
// @Summary Traffic sources
// @Description Returns traffic source breakdown.
// @Tags Insight
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]any
// @Router /api/insight/traffic-sources [get]
func (c *Controller) GetTrafficSources(ctx *gin.Context) {
	params := dto.OverviewParams{
		From: ctx.Query("from"),
		To:   ctx.Query("to"),
	}
	overview, err := c.svc.GetOverview(ctx.Request.Context(), params)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("analytics: GetTrafficSources failed")
		response.InternalErrorGin(ctx, err)
		return
	}

	response.SuccessGin(ctx, map[string]any{
		"sources": overview.TrafficSources,
	})
}

// GetInsights handles GET /api/insight/insights.
// @Summary Analytics insights
// @Description Returns AI-generated insights from analytics.
// @Tags Insight
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]any
// @Router /api/insight/insights [get]
func (c *Controller) GetInsights(ctx *gin.Context) {
	params := dto.OverviewParams{
		From: ctx.Query("from"),
		To:   ctx.Query("to"),
	}
	overview, err := c.svc.GetOverview(ctx.Request.Context(), params)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("analytics: GetInsights failed")
		response.InternalErrorGin(ctx, err)
		return
	}

	response.SuccessGin(ctx, map[string]any{
		"summary":         overview.Summary,
		"peak_hours":      overview.PeakHours,
		"traffic_sources": overview.TrafficSources,
		"interactions":    overview.Interactions,
		"devices":         overview.Devices,
	})
}

// Export handles GET /api/insight/export — returns CSV.
// @Summary Export analytics CSV
// @Description Exports analytics data as CSV.
// @Tags Insight
// @Produce text/csv
// @Security BearerAuth
// @Param from query string true "Start date"
// @Param to query string true "End date"
// @Success 200
// @Router /api/insight/export [get]
func (c *Controller) Export(ctx *gin.Context) {
	from := ctx.Query("from")
	to := ctx.Query("to")
	if from == "" || to == "" {
		response.BadRequestGin(ctx, fmt.Errorf("from and to are required"))
		return
	}

	csv, err := c.svc.ExportCSV(ctx.Request.Context(), from, to)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("analytics: Export failed")
		response.InternalErrorGin(ctx, err)
		return
	}

	ctx.Header("Content-Type", "text/csv")
	ctx.Header("Content-Disposition", "attachment; filename=\"traffic-report-"+from+"-to-"+to+".csv\"")
	ctx.Writer.WriteHeader(http.StatusOK)
	_, _ = ctx.Writer.Write([]byte(csv))
}

// GetUserJourney handles GET /api/insight/user-journey/:sessionId.
// @Summary Get user journey
// @Description Returns the event journey for a session.
// @Tags Insight
// @Produce json
// @Security BearerAuth
// @Param sessionId path string true "Session ID"
// @Success 200 {object} map[string]any
// @Router /api/insight/user-journey/{sessionId} [get]
func (c *Controller) GetUserJourney(ctx *gin.Context) {
	sessionID := ctx.Param("sessionId")
	if sessionID == "" {
		response.BadRequestGin(ctx, fmt.Errorf("sessionId is required"))
		return
	}

	journey, err := c.svc.GetUserJourney(ctx.Request.Context(), sessionID)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("analytics: GetUserJourney failed")
		response.InternalErrorGin(ctx, err)
		return
	}

	response.SuccessGin(ctx, journey)
}

// GetSession handles GET /api/insight/sessions/:sessionId.
// @Summary Get session detail
// @Description Returns session detail by ID.
// @Tags Insight
// @Produce json
// @Security BearerAuth
// @Param sessionId path string true "Session ID"
// @Success 200 {object} map[string]any
// @Router /api/insight/sessions/{sessionId} [get]
func (c *Controller) GetSession(ctx *gin.Context) {
	sessionID := ctx.Param("sessionId")
	if sessionID == "" {
		response.BadRequestGin(ctx, fmt.Errorf("sessionId is required"))
		return
	}

	session, err := c.svc.GetSession(ctx.Request.Context(), sessionID)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("analytics: GetSession failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	if session == nil {
		response.NotFoundGin(ctx, "session not found")
		return
	}

	response.SuccessGin(ctx, session)
}

// ─── Firebase Sync ─────────────────────────────────────────────────────────────

// firebaseAuthMiddleware validates the X-Firebase-API-Key header for server-to-server calls.
func (c *Controller) firebaseAuthMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		apiKey := ctx.GetHeader("X-Firebase-API-Key")
		if apiKey == "" || apiKey != c.cfg.Analytics.FirebaseAPIKey {
			c.log.WarnContext(ctx.Request.Context()).Str("ip", extractRealIP(ctx.Request)).Msg("analytics: firebase sync auth failed")
			response.UnauthorizedGin(ctx)
			ctx.Abort()
			return
		}
		ctx.Next()
	}
}

// SyncFirebaseEvents handles POST /api/insight/firebase/sync.
// @Summary Sync Firebase events
// @Description Imports Firebase events into analytics.
// @Tags Insight Firebase
// @Accept json
// @Produce json
// @Success 200 {object} map[string]any
// @Router /api/insight/firebase/sync [post]
func (c *Controller) SyncFirebaseEvents(ctx *gin.Context) {
	tenantID := tenant.FromContext(ctx.Request.Context())

	var req dto.SyncFirebaseEventsRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}

	result, err := c.svc.SyncFirebaseEvents(ctx.Request.Context(), tenantID, req)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("analytics: SyncFirebaseEvents failed")
		response.InternalErrorGin(ctx, err)
		return
	}

	c.log.InfoContext(ctx.Request.Context()).Int("synced", result.Synced).Int("duplicates", result.Duplicates).
		Msg("analytics: Firebase sync completed")
	response.SuccessGin(ctx, result)
}

// GetFirebaseSyncStats handles GET /api/insight/firebase/stats.
// @Summary Firebase sync stats
// @Description Returns Firebase sync statistics.
// @Tags Insight Firebase
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]any
// @Router /api/insight/firebase/stats [get]
func (c *Controller) GetFirebaseSyncStats(ctx *gin.Context) {
	params := dto.FirebaseSyncStatsParams{
		From: ctx.Query("from"),
		To:   ctx.Query("to"),
	}

	stats, err := c.svc.GetFirebaseSyncStats(ctx.Request.Context(), params.From, params.To)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("analytics: GetFirebaseSyncStats failed")
		response.InternalErrorGin(ctx, err)
		return
	}

	response.SuccessGin(ctx, stats)
}

// extractRealIP extracts the real client IP from request headers.
func extractRealIP(r *http.Request) string {
	if ip := r.Header.Get("CF-Connecting-IP"); ip != "" {
		return ip
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		return xrip
	}
	return r.RemoteAddr
}
